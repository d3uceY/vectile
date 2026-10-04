package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"vectile/backend/chunker"
	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
)

// indexItem is one thing to index — a file, a book, a commit — reduced to
// what storage needs: an identity, its chunks, and the metadata to record.
type indexItem struct {
	SourcePath string
	SourceType string
	Title      string
	Chunks     []chunker.Chunk
	Metadata   map[string]any
	FileHash   string
	Mtime      string
}

// itemBatch is a group of items whose chunks are embedded in one request.
type itemBatch struct {
	items []*indexItem
	texts []string    // all chunk texts, item then chunk order
	vecs  [][]float32 // filled by the embedding step; a nil entry = unembeddable chunk
}

// itemFunc returns the item at index i, or nil to skip it (unchanged, or
// nothing extractable). Items are built lazily so parsing only happens for
// items that will actually be embedded.
type itemFunc func(i int) *indexItem

// indexItemsBatched chunks, embeds, and stores a run of items. Embedding is
// sequential (llama.go serializes inference anyway) and every DB write happens
// on this goroutine, since SQLite takes no concurrent writers.
func indexItemsBatched(
	ctx context.Context,
	conn *sql.DB,
	cfg *config.Config,
	collectionID int64,
	label string,
	total int,
	itemAt itemFunc,
	embedder Embedder,
	result *IndexResult,
	progress ProgressCallback,
	preCleared bool,
) {
	if total == 0 {
		return
	}

	b := &itemBatcher{cfg: cfg, total: total, itemAt: itemAt}
	indexed := 0

	for {
		if ctx != nil && ctx.Err() != nil {
			return
		}
		batch := b.nextBatch(ctx)
		if batch == nil {
			return
		}

		// Embed the batch's texts in bounded slices. A single EmbedBatch over
		// the whole batch is one blocking llama.cpp call that can run for
		// minutes on CPU (a large batch, or one huge document with thousands
		// of chunks), during which a cancel would be ignored. Slicing gives a
		// cancel checkpoint every embedSliceCap texts with no throughput loss:
		// llama.cpp already batches internally by tokens and parallel
		// sequences, so the total work is identical either way.
		batch.vecs = make([][]float32, 0, len(batch.texts))
		for start := 0; start < len(batch.texts); start += embedSliceCap {
			if ctx != nil && ctx.Err() != nil {
				return
			}
			end := start + embedSliceCap
			if end > len(batch.texts) {
				end = len(batch.texts)
			}
			sliceVecs, err := embedder.EmbedBatch(batch.texts[start:end])
			if err != nil {
				// The slice failed as a batch — retry its texts one at a time
				// so a single unembeddable chunk only costs itself.
				sliceVecs = embedTextsIndividually(embedder, batch.texts[start:end])
			}
			batch.vecs = append(batch.vecs, sliceVecs...)
		}

		// Progress fires once per successfully indexed file so the frontend can
		// increment a per-collection count from a scoped event.
		for _, t := range writeItemBatch(conn, collectionID, batch, result, preCleared) {
			indexed++
			if progress != nil {
				progress(indexed, total, t)
			}
		}
	}
}

// embedSliceCap bounds how many chunk texts go into one EmbedBatch call.
// llama.cpp already processes a call's texts in parallel rounds of n_seq_max,
// so splitting a big batch into slices of this size does not change the total
// work — it only adds a cancellation checkpoint between slices, so a cancel
// lands within embedSliceCap chunks (~a few seconds on CPU) instead of after
// the whole batch.
const embedSliceCap = 8

// embedTextsIndividually retries one slice of a batch's texts one at a time,
// so a single unembeddable chunk only costs itself. Returns vectors aligned
// with texts; entries that still fail are nil, and writeItemBatch drops the
// whole item those chunks belong to (an item can't be stored partially).
func embedTextsIndividually(embedder Embedder, texts []string) [][]float32 {
	slog.Warn("embedding slice failed, retrying texts individually",
		"texts", len(texts))

	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := embedder.EmbedBatch([]string{t})
		if err != nil || len(v) != 1 {
			slog.Warn("skipping chunk that cannot be embedded", "err", err)
			continue
		}
		out[i] = v[0]
	}
	return out
}

// itemBatcher walks items in order, grouping them into batches of roughly
// embedding_batch_size chunks. An item's chunks are never split across batches.
type itemBatcher struct {
	cfg     *config.Config
	total   int
	next    int
	itemAt  itemFunc
}

// nextBatch returns the next batch, or nil once the items are exhausted or
// the run is cancelled mid-build (parsing items isn't preemptable, so the
// loop checks cancellation between items).
func (b *itemBatcher) nextBatch(ctx context.Context) *itemBatch {
	target := b.cfg.EmbeddingBatchSize
	if target < 1 {
		target = 1
	}

	batch := &itemBatch{}
	for b.next < b.total {
		if ctx != nil && ctx.Err() != nil {
			return nil
		}
		item := b.itemAt(b.next)
		b.next++

		if item == nil || !hasContent(item.Chunks) {
			// The itemFunc already recorded this as skipped or failed on the
			// result; counting it again here would double-report it.
			continue
		}

		batch.items = append(batch.items, item)
		for _, c := range item.Chunks {
			batch.texts = append(batch.texts, c.Text)
		}
		if len(batch.texts) >= target {
			return batch
		}
	}
	if len(batch.items) > 0 {
		return batch
	}
	return nil
}

// hasContent reports whether any chunk carries non-blank text.
func hasContent(chunks []chunker.Chunk) bool {
	for _, c := range chunks {
		if strings.TrimSpace(c.Text) != "" {
			return true
		}
	}
	return false
}

// hasNilVec reports whether any embedding in vecs is nil (a chunk that could
// not be embedded).
func hasNilVec(vecs [][]float32) bool {
	for _, v := range vecs {
		if v == nil {
			return true
		}
	}
	return false
}

// writeItemBatch stores an embedded batch, attributing failures per item so
// one bad item does not sink the rest. It returns a display name for each item
// that was successfully stored, so the caller can emit per-file progress.
func writeItemBatch(conn *sql.DB, collectionID int64, b *itemBatch, result *IndexResult, preCleared bool) []string {
	// Offsets into b.vecs are only meaningful if one vector came back per text.
	if len(b.vecs) != len(b.texts) {
		msg := fmt.Sprintf("embedding count mismatch: got %d vectors for %d chunks", len(b.vecs), len(b.texts))
		slog.Error(msg)
		result.Errors += len(b.items)
		result.ErrorMessages = append(result.ErrorMessages, msg)
		return nil
	}

	if !preCleared {
		purgeSourceDocuments(conn, collectionID, b.items)
	}

	var indexedNames []string
	offset := 0
	for _, item := range b.items {
		vecs := b.vecs[offset : offset+len(item.Chunks)]
		offset += len(item.Chunks)

		// A nil vector means one of the item's chunks couldn't be embedded.
		// An item can only be stored whole (storeItem writes all its chunks'
		// vectors), so a partially-embedded item is dropped entirely.
		if hasNilVec(vecs) {
			result.Errors++
			if result.Errors <= 10 {
				msg := fmt.Sprintf("item %s has unembeddable chunk(s), skipping", item.SourcePath)
				slog.Warn(msg)
				result.ErrorMessages = append(result.ErrorMessages, msg)
			}
			continue
		}

		if err := storeItem(conn, collectionID, item, vecs); err != nil {
			result.Errors++
			if result.Errors <= 10 {
				msg := fmt.Sprintf("error indexing %s %s: %v", item.SourceType, item.SourcePath, err)
				slog.Warn(msg)
				result.ErrorMessages = append(result.ErrorMessages, msg)
			}
			continue
		}
		result.Indexed++
		indexedNames = append(indexedNames, itemProgressName(item))
	}
	return indexedNames
}

// itemProgressName is the label the per-file progress event carries for a
// stored item: its display title when it has one (a book title, a
// repo-relative path), otherwise the file's base name. Item builders for
// obsidian vaults and project folders set no Title, so without the fallback
// the loader shows a blank line under the bar for the two source types people
// index most.
func itemProgressName(item *indexItem) string {
	if item.Title != "" {
		return item.Title
	}
	if item.SourcePath == "" {
		return ""
	}
	return filepath.Base(item.SourcePath)
}

// sqlParamLimit bounds how many bind parameters go into one statement.
const sqlParamLimit = 500

// purgeSourceDocuments removes the existing documents and embeddings for every
// item in a batch, in as few statements as possible. Batched because the vec0
// tables have no index on document_id, so per-item deletes would each scan the
// whole vector table.
func purgeSourceDocuments(conn *sql.DB, collectionID int64, items []*indexItem) {
	if len(items) == 0 {
		return
	}

	var docIDs []any
	for start := 0; start < len(items); start += sqlParamLimit {
		end := start + sqlParamLimit
		if end > len(items) {
			end = len(items)
		}
		args := make([]any, 0, len(items[start:end])+1)
		args = append(args, collectionID)
		for _, item := range items[start:end] {
			args = append(args, item.SourcePath)
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", end-start), ",")

		rows, err := conn.Query(`
			SELECT d.id FROM documents d
			JOIN sources s ON d.source_id = s.id
			WHERE s.collection_id = ? AND s.source_path IN (`+placeholders+`)`, args...)
		if err != nil {
			slog.Warn("cannot list documents to purge", "err", err)
			return
		}
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				docIDs = append(docIDs, id)
			}
		}
		rows.Close()
	}

	if len(docIDs) == 0 {
		return
	}

	for start := 0; start < len(docIDs); start += sqlParamLimit {
		end := start + sqlParamLimit
		if end > len(docIDs) {
			end = len(docIDs)
		}
		chunk := docIDs[start:end]
		if err := db.DeleteEmbeddings(conn, chunk); err != nil {
			slog.Warn("cannot delete embeddings", "err", err)
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		if _, err := conn.Exec("DELETE FROM documents WHERE id IN ("+placeholders+")", chunk...); err != nil {
			slog.Warn("cannot delete documents", "err", err)
		}
	}
}

// storeItem writes one item's chunks and their embeddings. vecs is parallel
// to item.Chunks. purged=true skips the per-source cleanup because the batch
// was already purged.
func storeItem(conn *sql.DB, collectionID int64, item *indexItem, vecs [][]float32) error {
	sourceID, err := upsertSourceRow(conn, collectionID, item.SourcePath, item.SourceType,
		item.FileHash, item.Mtime, true)
	if err != nil {
		return err
	}

	for i, c := range item.Chunks {
		metaJSON, err := chunkMetadata(item, c)
		if err != nil {
			return err
		}
		title := c.Title
		if item.Title != "" {
			title = item.Title
		}

		docRes, err := conn.Exec(
			"INSERT INTO documents (source_id, collection_id, chunk_index, title, content, metadata) VALUES (?, ?, ?, ?, ?, ?)",
			sourceID, collectionID, c.ChunkIndex, title, c.Text, metaJSON,
		)
		if err != nil {
			return fmt.Errorf("insert document: %w", err)
		}
		docID, _ := docRes.LastInsertId()
		if err := db.InsertEmbedding(conn, docID, embeddings.SerializeFloat32(vecs[i])); err != nil {
			return fmt.Errorf("insert embedding: %w", err)
		}
	}
	return nil
}

// chunkMetadata merges item-wide metadata with the chunk's own; chunk keys win.
func chunkMetadata(item *indexItem, c chunker.Chunk) (string, error) {
	if len(item.Metadata) == 0 && len(c.Metadata) == 0 {
		return "", nil
	}
	merged := make(map[string]any, len(item.Metadata)+len(c.Metadata))
	for k, v := range item.Metadata {
		merged[k] = v
	}
	for k, v := range c.Metadata {
		merged[k] = v
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(b), nil
}
