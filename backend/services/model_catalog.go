package services

// CatalogModel is one pre-approved embedding model vectile can fetch for the
// user. Each pins the SHA-256 and the exact byte size of the file at its URL,
// and the download refuses to install anything that does not match both, so a
// corrupted or tampered file is never loaded. Each also carries a link out to
// the upstream page for users who prefer to read about or inspect it.
type CatalogModel struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	File         string `json:"file"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	Dimensions   int    `json:"dimensions"`
	SizeBytes    int64  `json:"sizeBytes"` // exact, not rounded
	Quantization string `json:"quantization"`
	Language     string `json:"language"`
	Recommended  bool   `json:"recommended"`
	Description  string `json:"description"`
	HFURL        string `json:"hfUrl"`
}

// modelCatalog is the list of models offered in the app. The default
// recommendation is bge-small-en-v1.5 Q8_0: full Q8 precision for ~37 MB.
//
// The URLs use Hugging Face's resolve/main, which is a moving ref, so the
// SHA-256 and size recorded here are what actually pin a version. They come
// from the repo's file listing (the lfs.oid field of
// https://huggingface.co/api/models/<repo>/tree/main); refresh them together
// with any URL change.
var modelCatalog = []CatalogModel{
	{
		Key: "bge-small-en-v1.5-q8_0", Name: "BGE Small EN v1.5", File: "bge-small-en-v1.5-q8_0.gguf",
		URL:       "https://huggingface.co/ggml-org/bge-small-en-v1.5-Q8_0-GGUF/resolve/main/bge-small-en-v1.5-q8_0.gguf?download=true",
		SHA256:    "f046db1dc724cf4f6f0a0c5917e922823b73eb1d27b8f9a9c2797f7866974804",
		Dimensions: 384, SizeBytes: 36685152, Quantization: "Q8_0", Language: "English",
		Recommended: true, Description: "Best quality for the size.",
		HFURL: "https://huggingface.co/ggml-org/bge-small-en-v1.5-Q8_0-GGUF",
	},
	{
		Key: "bge-small-en-v1.5-q4_k_m", Name: "BGE Small EN v1.5", File: "bge-small-en-v1.5-q4_k_m.gguf",
		URL:       "https://huggingface.co/CompendiumLabs/bge-small-en-v1.5-gguf/resolve/main/bge-small-en-v1.5-q4_k_m.gguf?download=true",
		SHA256:    "363a0a4855dff6c653e06efe3209157debcf7f74e52d0d7c71e2747cd523043e",
		Dimensions: 384, SizeBytes: 24808576, Quantization: "Q4_K_M", Language: "English",
		Description: "Smallest of the English options.",
		HFURL: "https://huggingface.co/CompendiumLabs/bge-small-en-v1.5-gguf",
	},
	{
		Key: "bge-m3-q4_k_m", Name: "BGE-M3", File: "bge-m3-Q4_K_M.gguf",
		URL:       "https://huggingface.co/lm-kit/bge-m3-gguf/resolve/main/bge-m3-Q4_K_M.gguf?download=true",
		SHA256:    "e251234fcb7d050991a6be491952f485bf5c641dd10c3272dc1301fd281ad50f",
		Dimensions: 1024, SizeBytes: 437778592, Quantization: "Q4_K_M", Language: "Multilingual",
		Description: "Best semantic quality.",
		HFURL: "https://huggingface.co/lm-kit/bge-m3-gguf",
	},
	{
		Key: "all-minilm-l6-v2-q4_k_m", Name: "all-MiniLM-L6-v2", File: "all-MiniLM-L6-v2-Q4_K_M.gguf",
		URL:       "https://huggingface.co/second-state/All-MiniLM-L6-v2-Embedding-GGUF/resolve/main/all-MiniLM-L6-v2-Q4_K_M.gguf?download=true",
		SHA256:    "2ec4cee28a27a9c973d5f5230930d6ef6e52694bd2bc71be26a9bef5b1d755e6",
		Dimensions: 384, SizeBytes: 20999104, Quantization: "Q4_K_M", Language: "English",
		Description: "Very lightweight.",
		HFURL: "https://huggingface.co/second-state/All-MiniLM-L6-v2-Embedding-GGUF",
	},
}

// ModelCatalog returns the list of models offered for download.
func ModelCatalog() []CatalogModel { return modelCatalog }

// CatalogByKey looks up one catalog model by its key.
func CatalogByKey(key string) (CatalogModel, bool) {
	for _, m := range modelCatalog {
		if m.Key == key {
			return m, true
		}
	}
	return CatalogModel{}, false
}