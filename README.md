# Linotype

Linotype is a Go command-line tool that extracts PGS (Presentation Graphic Stream) bitmap subtitles from MKV containers or raw `.sup` files, runs OCR on individual subtitle bitmaps using any OpenAI-compatible vision LLM endpoint, and outputs text-based subtitle files in `.ass` or `.webvtt` formats with proper screen positioning and colors.

## Features

- **MKV & SUP Input**: Directly process `.sup` files or automatically demux PGS tracks from `.mkv` files using `ffmpeg`.
- **Multi-Track Support**: Automatically extracts and converts all PGS subtitle tracks found in an MKV.
- **Vision LLM OCR**: Works with any OpenAI-compatible vision API (OpenAI, vLLM, Ollama, LiteLLM, OpenRouter, etc.).
- **Precise Positioning**:
  - **ASS**: Generates `\an1`–`\an9` alignment tags and pixel-accurate `\pos(x,y)` tags matching the video reference frame.
  - **WebVTT**: Generates `line:N%`, `position:N%`, and `align:start|center|end` cue settings.
- **Color Preservation**: Extracts dominant text colors from the PGS palette (PDS) and maps them to ASS `\c` tags or WebVTT color formatting.
- **Concurrent Worker Pool**: Configurable parallelism with `-workers N` (default 4) and ordered subtitle reassembly.
- **Fail-Fast Error Handling**: Immediately halts on API failures to prevent runaway costs or token consumption.
- **TUI Progress Bar**: Real-time progress display with current/total count, percentage, and ETA using Charmbracelet libraries; easily silenced via `-quiet`.
- **Reusable Packages**: Modular architecture with pure Go PGS parser in `pkg/pgs`.

---

## Prerequisites

- **Go 1.22+** (to build from source)
- **`ffmpeg` and `ffprobe`**: Must be installed and on your system `PATH` if processing `.mkv` files. Raw `.sup` files do not require FFmpeg.

---

## Installation

```bash
git clone https://git.dyson.somuchcrypto.com/gearnsc/linotype.git
cd linotype
go build -o linotype .
```

---

## Configuration

Linotype searches for a YAML config file in the following order:
1. Path provided by `-config <path>`
2. `./linotype.yaml` (current working directory)
3. `~/.config/linotype/config.yaml`

Copy the provided example config:

```bash
cp linotype.yaml.example linotype.yaml
```

Example `linotype.yaml`:

```yaml
api:
  base_url: "https://api.openai.com/v1"
  api_key: "sk-..."
  model: "gpt-4o"

ocr:
  language: "English"
  # Optional custom system prompt ({language} placeholder is replaced):
  # prompt: |
  #   You are an OCR engine. The image contains subtitle text in {language}.
  #   Return ONLY the exact text visible in the image, preserving line breaks.

workers: 4
format: "ass"
```

### Environment Variables

You can also set API settings via environment variables:
- `LINOTYPE_API_BASE`
- `LINOTYPE_API_KEY`
- `LINOTYPE_MODEL`

---

## CLI Usage

```bash
linotype [flags] <input.mkv|input.sup>
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-format` | `ass` | Output subtitle format: `ass` or `webvtt` |
| `-workers` | `4` | Number of concurrent OCR workers |
| `-language` | `English` | Language hint included in the OCR prompt |
| `-config` | `""` | Path to custom YAML configuration file |
| `-quiet` | `false` | Suppress TUI progress bar and decorative output |
| `-api-base` | `""` | Override API base URL |
| `-api-key` | `""` | Override API key |
| `-model` | `""` | Override model name |

### Examples

**Convert a raw `.sup` file to ASS:**
```bash
linotype -api-key "sk-..." subs.sup
# Produces: subs.ass
```

**Convert an MKV file to WebVTT with 8 workers:**
```bash
linotype -format webvtt -workers 8 movie.mkv
# Produces: movie.track0.eng.vtt, movie.track1.jpn.vtt, etc.
```

**Using a local Ollama or vLLM vision model:**
```bash
linotype -api-base "http://localhost:11434/v1" -model "llava" -api-key "none" subs.sup
```

**Automated / Scripted run (silent progress bar):
```bash
linotype -quiet -config /etc/linotype.yaml input.mkv
```

---

## Architecture

- [`pkg/pgs`](file:///d:/code/linotoype/pkg/pgs): Pure Go binary parser for PGS/SUP streams, BT.709 palette conversion, RLE bitmap decoding, multi-object compositing, and DisplaySet pairing.
- [`pkg/ocr`](file:///d:/code/linotoype/pkg/ocr): OpenAI-compatible Chat Completions client sending base64 PNG data to vision models.
- [`pkg/ass`](file:///d:/code/linotoype/pkg/ass): ASS subtitle file generator with `\pos(x,y)`, `\an`, and `\c` tags.
- [`pkg/webvtt`](file:///d:/code/linotoype/pkg/webvtt): WebVTT generator with `line`, `position`, and `align` cues.
- [`pkg/extract`](file:///d:/code/linotoype/pkg/extract): FFmpeg/FFprobe wrapper for discovering and demuxing PGS subtitle streams from MKVs.
- [`pkg/config`](file:///d:/code/linotoype/pkg/config): YAML configuration loader with default fallbacks and cascading overrides.
- [`pkg/subtitle`](file:///d:/code/linotoype/pkg/subtitle): Common data contracts bridging parser, OCR, and writers.

