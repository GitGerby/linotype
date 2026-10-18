package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/ass"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/config"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/extract"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/ocr"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/pgs"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/subtitle"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/vobsub"
	"git.dyson.somuchcrypto.com/gearnsc/linotype/pkg/webvtt"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	errStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF0055"))
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %v\n", errStyle.Render("Error:"), err)
		os.Exit(1)
	}
}

func run() error {
	formatFlag := flag.String("format", "webvtt", "Output subtitle format: ass or webvtt (default: from config or webvtt)")
	workersFlag := flag.Int("workers", 0, "Number of concurrent OCR workers (default: from config or 4)")
	languageFlag := flag.String("language", "", "Language hint for OCR (default: from config or English)")
	configFlag := flag.String("config", "", "Path to YAML config file")
	quietFlag := flag.Bool("quiet", false, "Suppress TUI progress bar and decorative output")
	apiBaseFlag := flag.String("api-base", "", "OpenAI-compatible API base URL override")
	apiKeyFlag := flag.String("api-key", "", "API key override")
	modelFlag := flag.String("model", "", "Model name override")
	debugFlag := flag.Bool("debug", false, "Enable debug mode")
	pngExportFlag := flag.String("png_export", "", "Directory to export copies of PNG images sent to LLM (requires --debug)")
	flag.StringVar(pngExportFlag, "png-export", "", "Alias for --png_export")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <input.mkv|input.vob|input.sup|input.idx|input.sub>\n\nFlags:\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		return fmt.Errorf("missing input file argument")
	}

	inputPath := flag.Arg(0)

	cfg, err := config.Load(*configFlag)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if env := os.Getenv("LINOTYPE_API_BASE"); env != "" {
		cfg.API.BaseURL = env
	}
	if env := os.Getenv("LINOTYPE_API_KEY"); env != "" {
		cfg.API.APIKey = env
	}
	if env := os.Getenv("LINOTYPE_MODEL"); env != "" {
		cfg.API.Model = env
	}
	if env := os.Getenv("LINOTYPE_DEBUG"); env != "" {
		cfg.Debug = env == "true" || env == "1"
	}
	if env := os.Getenv("LINOTYPE_PNG_EXPORT"); env != "" {
		cfg.PNGExport = env
	}

	providedFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		providedFlags[f.Name] = true
	})

	if providedFlags["format"] && *formatFlag != "" {
		cfg.Format = strings.ToLower(*formatFlag)
	}
	if providedFlags["workers"] && *workersFlag > 0 {
		cfg.Workers = *workersFlag
	}
	if providedFlags["language"] && *languageFlag != "" {
		cfg.OCR.Language = *languageFlag
	}
	if providedFlags["api-base"] && *apiBaseFlag != "" {
		cfg.API.BaseURL = *apiBaseFlag
	}
	if providedFlags["api-key"] && *apiKeyFlag != "" {
		cfg.API.APIKey = *apiKeyFlag
	}
	if providedFlags["model"] && *modelFlag != "" {
		cfg.API.Model = *modelFlag
	}
	if providedFlags["debug"] {
		cfg.Debug = *debugFlag
	}
	if providedFlags["png_export"] || providedFlags["png-export"] {
		cfg.PNGExport = *pngExportFlag
	}

	if cfg.Format != "ass" && cfg.Format != "webvtt" {
		return fmt.Errorf("unsupported output format %q (must be 'ass' or 'webvtt')", cfg.Format)
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
	})
	if cfg.Debug {
		logger.SetLevel(log.DebugLevel)
	} else if *quietFlag {
		logger.SetLevel(log.WarnLevel)
	}

	if cfg.Debug && cfg.PNGExport != "" {
		logger.Debug("PNG debug export enabled", "dir", cfg.PNGExport)
	} else if !cfg.Debug && cfg.PNGExport != "" {
		logger.Warn("--png_export specified without --debug; PNG export is disabled")
	}

	if !*quietFlag {
		fmt.Fprintf(os.Stderr, "%s — Bitmap Subtitle OCR Converter (PGS & VOBSUB)\n\n", titleStyle.Render("Linotype"))
	}

	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("input file not found: %w", err)
	}

	ocrClient := ocr.NewClient(cfg.API.BaseURL, cfg.API.APIKey, cfg.API.Model)
	ocrClient.Debug = cfg.Debug
	ocrClient.PNGExport = cfg.PNGExport
	prompt := ocr.BuildPrompt(cfg.OCR.Language, cfg.OCR.Prompt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ext := strings.ToLower(filepath.Ext(inputPath))
	switch ext {
	case ".mkv", ".vob":
		return processContainer(ctx, inputPath, cfg, ocrClient, prompt, *quietFlag, logger)
	case ".sup":
		outPath := getOutputFilename(inputPath, -1, "", cfg.Format)
		return processSUP(ctx, inputPath, outPath, cfg, ocrClient, prompt, *quietFlag, logger, Dimensions{})
	case ".idx":
		return processIDX(ctx, inputPath, cfg, ocrClient, prompt, *quietFlag, logger)
	case ".sub":
		return processSUB(ctx, inputPath, cfg, ocrClient, prompt, *quietFlag, logger)
	default:
		return fmt.Errorf("unsupported input file format %q (expected .mkv, .vob, .sup, .idx, or .sub)", ext)
	}
}

func processContainer(ctx context.Context, mediaPath string, cfg *config.Config, ocrClient *ocr.Client, prompt string, quiet bool, logger *log.Logger) error {
	logger.Info("Inspecting container tracks", "file", mediaPath)

	vidW, vidH, err := extract.GetVideoDimensions(ctx, mediaPath)
	if err != nil {
		logger.Warn("Could not detect video dimensions, using subtitle stream dimensions", "err", err)
	} else if vidW > 0 && vidH > 0 {
		logger.Info("Detected video dimensions", "width", vidW, "height", vidH)
	}

	tracks, err := extract.ListSubtitleTracks(ctx, mediaPath)
	if err != nil {
		return fmt.Errorf("failed to list subtitle tracks: %w", err)
	}

	if len(tracks) == 0 {
		return fmt.Errorf("no subtitle tracks found in %s", mediaPath)
	}

	logger.Info("Found subtitle tracks", "count", len(tracks))
	for _, t := range tracks {
		logger.Info("Track", "index", t.Index, "relative_index", t.RelIndex, "language", t.Language, "codec", string(t.Codec))
	}

	for _, track := range tracks {
		if err := processContainerTrack(ctx, mediaPath, track, cfg, ocrClient, prompt, quiet, logger, vidW, vidH); err != nil {
			return err
		}
	}
	return nil
}

func processContainerTrack(ctx context.Context, mediaPath string, track extract.Track, cfg *config.Config, ocrClient *ocr.Client, prompt string, quiet bool, logger *log.Logger, vidW, vidH int) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	switch track.Codec {
	case extract.CodecPGS:
		tempSup, err := os.CreateTemp("", fmt.Sprintf("linotype_track%d_*.sup", track.RelIndex))
		if err != nil {
			return err
		}
		tempSupPath := tempSup.Name()
		tempSup.Close()
		defer os.Remove(tempSupPath)

		logger.Info("Extracting PGS track", "track", track.RelIndex, "lang", track.Language)
		if err := extract.ExtractPGS(ctx, mediaPath, track.RelIndex, tempSupPath); err != nil {
			return err
		}

		outPath := getOutputFilename(mediaPath, track.RelIndex, track.Language, cfg.Format)
		if err := processSUP(ctx, tempSupPath, outPath, cfg, ocrClient, prompt, quiet, logger, Dimensions{Width: vidW, Height: vidH}); err != nil {
			return err
		}
	case extract.CodecVobSub:
		tempSub, err := os.CreateTemp("", fmt.Sprintf("linotype_track%d_*.sub", track.RelIndex))
		if err != nil {
			return err
		}
		tempSubPath := tempSub.Name()
		tempSub.Close()
		defer os.Remove(tempSubPath)

		logger.Info("Extracting DVD Subtitle track", "track", track.RelIndex, "lang", track.Language)
		if err := extract.ExtractDVDSubtitle(ctx, mediaPath, track.RelIndex, tempSubPath); err != nil {
			return err
		}

		// Parse codec private or use fallback palette
		palette := vobsub.DefaultPalette()
		subW := track.Width
		subH := track.Height

		var codecPriv []byte
		if len(track.Extradata) > 0 {
			codecPriv = track.Extradata
		} else {
			cp, err := extract.GetTrackCodecPrivate(ctx, mediaPath, track.RelIndex)
			if err == nil {
				codecPriv = cp
			}
		}

		if len(codecPriv) > 0 {
			idxInfo, err := vobsub.ParseIdx(bytes.NewReader(codecPriv))
			if err == nil {
				if idxInfo.Palette != nil {
					palette = idxInfo.Palette
				}
				if idxInfo.Width > 0 && idxInfo.Height > 0 {
					subW = idxInfo.Width
					subH = idxInfo.Height
				}
			}
		}

		if subW <= 0 || subH <= 0 {
			subW = 720
			subH = 480
		}

		f, err := os.Open(tempSubPath)
		if err != nil {
			return err
		}
		defer f.Close()

		items, err := vobsub.ParseStream(f, palette, subW, subH)
		if err != nil {
			return err
		}

		outPath := getOutputFilename(mediaPath, track.RelIndex, track.Language, cfg.Format)
		if err := processSubtitleItems(ctx, items, outPath, cfg, ocrClient, prompt, quiet, logger, Dimensions{Width: vidW, Height: vidH}); err != nil {
			return err
		}
	}
	return nil
}

func processIDX(ctx context.Context, idxPath string, cfg *config.Config, ocrClient *ocr.Client, prompt string, quiet bool, logger *log.Logger) error {
	subPath := strings.TrimSuffix(idxPath, filepath.Ext(idxPath)) + ".sub"
	if _, err := os.Stat(subPath); err != nil {
		return fmt.Errorf("companion .sub file not found for .idx")
	}

	idxFile, err := os.Open(idxPath)
	if err != nil {
		return err
	}
	defer idxFile.Close()

	idxInfo, err := vobsub.ParseIdx(idxFile)
	if err != nil {
		return err
	}

	subFile, err := os.Open(subPath)
	if err != nil {
		return err
	}
	defer subFile.Close()

	for _, track := range idxInfo.Tracks {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		items, err := vobsub.ParseIndexedWithInfo(idxInfo, subFile, track.Index)
		if err != nil {
			logger.Warn("Failed to parse track", "track", track.Index, "err", err)
			continue
		}

		outPath := getOutputFilename(idxPath, track.Index, track.Language, cfg.Format)
		targetDim := Dimensions{}
		if idxInfo.Width > 0 && idxInfo.Height > 0 {
			targetDim = Dimensions{Width: idxInfo.Width, Height: idxInfo.Height}
		}
		if err := processSubtitleItems(ctx, items, outPath, cfg, ocrClient, prompt, quiet, logger, targetDim); err != nil {
			return err
		}
	}
	return nil
}

func processSUB(ctx context.Context, subPath string, cfg *config.Config, ocrClient *ocr.Client, prompt string, quiet bool, logger *log.Logger) error {
	idxPath := strings.TrimSuffix(subPath, filepath.Ext(subPath)) + ".idx"
	if _, err := os.Stat(idxPath); err == nil {
		return processIDX(ctx, idxPath, cfg, ocrClient, prompt, quiet, logger)
	}

	f, err := os.Open(subPath)
	if err != nil {
		return err
	}
	defer f.Close()

	items, err := vobsub.ParseStream(f, vobsub.DefaultPalette(), 720, 480)
	if err != nil {
		return err
	}

	outPath := getOutputFilename(subPath, -1, "", cfg.Format)
	return processSubtitleItems(ctx, items, outPath, cfg, ocrClient, prompt, quiet, logger, Dimensions{Width: 720, Height: 480})
}

func processSUP(ctx context.Context, supPath, outPath string, cfg *config.Config, ocrClient *ocr.Client, prompt string, quiet bool, logger *log.Logger, targetDim Dimensions) error {
	logger.Info("Parsing PGS subtitle stream", "file", supPath)

	f, err := os.Open(supPath)
	if err != nil {
		return fmt.Errorf("failed to open SUP file: %w", err)
	}
	defer f.Close()

	sets, err := pgs.Parse(f)
	if err != nil {
		return fmt.Errorf("failed to parse SUP stream: %w", err)
	}

	logger.Info("Parsed display sets", "total", len(sets))

	items, err := pgs.ExtractSubtitleItems(sets)
	if err != nil {
		return fmt.Errorf("failed to extract subtitle items: %w", err)
	}

	return processSubtitleItems(ctx, items, outPath, cfg, ocrClient, prompt, quiet, logger, targetDim)
}

type Dimensions struct {
	Width  int
	Height int
}

func (d Dimensions) IsZero() bool {
	return d.Width <= 0 || d.Height <= 0
}

func processSubtitleItems(ctx context.Context, items []subtitle.Item, outPath string, cfg *config.Config, ocrClient *ocr.Client, prompt string, quiet bool, logger *log.Logger, targetDim Dimensions) error {
	totalItems := len(items)
	if totalItems == 0 {
		logger.Warn("No visible subtitle items found in stream")
		return writeOutput(outPath, cfg.Format, nil)
	}

	logger.Info("Extracted subtitle images for OCR", "count", totalItems, "workers", cfg.Workers)

	ocrCtx, cancelOCR := context.WithCancel(ctx)
	defer cancelOCR()

	type job struct {
		item subtitle.Item
	}

	type result struct {
		index int
		text  string
		err   error
	}

	jobs := make(chan job, totalItems)
	results := make(chan result, totalItems)

	for _, item := range items {
		jobs <- job{item: item}
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < cfg.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if ocrCtx.Err() != nil {
					return
				}

				text, err := ocrClient.Recognize(ocrCtx, j.item.PNGData, prompt)
				if err != nil {
					results <- result{index: j.item.Index, err: err}
					cancelOCR()
					return
				}
				results <- result{index: j.item.Index, text: text}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var prog progress.Model
	if !quiet {
		prog = progress.New(
			progress.WithDefaultGradient(),
			progress.WithWidth(30),
		)
	}

	textByIndex := make(map[int]string, totalItems)
	completed := 0
	startTime := time.Now()
	var ocrErr error

	for res := range results {
		if res.err != nil {
			if ocrErr == nil {
				ocrErr = res.err
			}
			continue
		}

		textByIndex[res.index] = res.text
		completed++

		if !quiet {
			pct := float64(completed) / float64(totalItems)
			elapsed := time.Since(startTime)
			var etaStr string
			if completed > 0 && pct < 1.0 {
				totalEst := time.Duration(float64(elapsed) / pct)
				eta := totalEst - elapsed
				etaStr = fmt.Sprintf("ETA: %s", eta.Round(time.Second))
			} else {
				etaStr = "Done"
			}

			bar := prog.ViewAs(pct)
			fmt.Fprintf(os.Stderr, "\rOCR Progress: %s %d/%d (%0.1f%%) | %s   ",
				bar, completed, totalItems, pct*100, etaStr)
		}
	}

	if !quiet {
		fmt.Fprintln(os.Stderr)
	}

	if ocrErr != nil {
		return fmt.Errorf("OCR failed: %w", ocrErr)
	}
	if ocrCtx.Err() != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	events := make([]subtitle.Event, len(items))
	for i, item := range items {
		x := item.X
		y := item.Y
		w := item.Width
		h := item.Height
		vw := item.VideoWidth
		vh := item.VideoHeight

		if vw <= 0 {
			if x+w <= 720 && y+h <= 576 {
				vw = 720
			} else {
				vw = 1920
			}
		}
		if vh <= 0 {
			if x+w <= 720 && y+h <= 480 {
				vh = 480
			} else if x+w <= 720 && y+h <= 576 {
				vh = 576
			} else {
				vh = 1080
			}
		}

		if !targetDim.IsZero() && (vw != targetDim.Width || vh != targetDim.Height) {
			if vw > 0 && vh > 0 {
				scaleX := float64(targetDim.Width) / float64(vw)
				scaleY := float64(targetDim.Height) / float64(vh)
				x = int(math.Round(float64(x) * scaleX))
				y = int(math.Round(float64(y) * scaleY))
				w = int(math.Round(float64(w) * scaleX))
				h = int(math.Round(float64(h) * scaleY))
			}
			vw = targetDim.Width
			vh = targetDim.Height
		}

		events[i] = subtitle.Event{
			Index:       item.Index,
			StartTime:   item.StartTime,
			EndTime:     item.EndTime,
			Text:        textByIndex[item.Index],
			X:           x,
			Y:           y,
			Width:       w,
			Height:      h,
			VideoWidth:  vw,
			VideoHeight: vh,
			Color:       item.Color,
		}
	}

	if err := writeOutput(outPath, cfg.Format, events); err != nil {
		return fmt.Errorf("failed to write subtitle file: %w", err)
	}

	logger.Info("Subtitle file generated successfully",
		"output", outPath,
		"format", cfg.Format,
		"subtitles", len(events),
		"time", time.Since(startTime).Round(time.Millisecond))

	if !quiet {
		fmt.Printf("%s Saved to %s (%d subtitles)\n",
			infoStyle.Render("✓"), outPath, len(events))
	}

	return nil
}

func writeOutput(outPath, format string, events []subtitle.Event) error {
	outDir := filepath.Dir(outPath)
	if outDir != "" && outDir != "." {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return err
		}
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	switch format {
	case "ass":
		return ass.Write(outFile, events)
	case "webvtt":
		return webvtt.Write(outFile, events)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

func getOutputFilename(inputPath string, trackRelIndex int, lang, format string) string {
	ext := "." + format
	if format == "webvtt" {
		ext = ".vtt"
	}

	dir := filepath.Dir(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))

	if trackRelIndex >= 0 {
		if lang != "" {
			return filepath.Join(dir, fmt.Sprintf("%s.track%d.%s%s", base, trackRelIndex, lang, ext))
		}
		return filepath.Join(dir, fmt.Sprintf("%s.track%d%s", base, trackRelIndex, ext))
	}

	return filepath.Join(dir, base+ext)
}
