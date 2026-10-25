---
sidebar_position: 5
---

# Models

vectile needs one file it cannot supply: a model that turns text into numbers. It is a `.gguf` file that runs inside the app through llama.cpp, so there is no separate program to start.

Models live in `models/*.gguf` inside vectile's app data folder.

## Downloading one

**Settings → Model → Get a model** lists a short catalog vectile can check after downloading.

| Model | Quantization | Size | Dimensions | Language |
|---|---|---|---|---|
| BGE Small EN v1.5 | Q8_0 | 37 MB | 384 | English |
| BGE Small EN v1.5 | Q4_K_M | 25 MB | 384 | English |
| BGE-M3 | Q4_K_M | 438 MB | 1024 | Multilingual |
| all-MiniLM-L6-v2 | Q4_K_M | 21 MB | 384 | English |

Dimensions are how many numbers describe each passage. Take BGE Small EN v1.5 at Q8_0 unless your library is not in English: 8-bit precision in a 37 MB download. BGE-M3 is the one for other languages, at about twelve times the size and a noticeable cost in index time.

![Settings, Model section: the loaded model state, the active model dropdown, Import model, the per-model batch size, the Get a model catalog, and the installed model list](/img/screenshots/settings.png)

## Importing your own

**Import model…** copies a `.gguf` file into the models folder. A `.gguf` dropped into the folder by hand is picked up automatically. A chat model renamed to `.gguf` is rejected when you import it.

## Switching the active model

Any switch made while something is indexed needs a re-index. Two models can share a dimension and still describe a passage differently, so stored vectors cannot be compared against the new model's. vectile asks before clearing them, then rebuilds the vector tables; Library shows **needs reindex** until the collections are re-indexed. Keyword search keeps working in the meantime.

Re-applying the active model changes nothing, and a switch on an empty library applies immediately.

Switching also empties the [query cache](./search#the-query-cache): a cached query from the old model means nothing to the new one.

## Per-model settings

| Setting | Default | What it does |
|---|---|---|
| Context window | 2048 tokens | The most text the model reads at once. |
| Batch size | 32 chunks | How many passages are sent to the model per call. |
| Threads | 0, meaning all cores | How many CPU threads the model may use. |

Changes to the active model apply immediately.

:::tip
If indexing makes the machine sluggish, set threads to half your cores rather than lowering the batch size. Batch size changes how fast the work gets done, not how much CPU it takes.
:::

## Uninstalling

Deleting a model removes the file from the models folder. The active model cannot be deleted; switch to another one first.
