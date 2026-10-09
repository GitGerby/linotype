package main

import (
	"context"
	"flag"
	"fmt"
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
	formatFlag := flag.String("format", "", "Output subtitle format: ass or webvtt (default: from config or ass)")
	workersFlag := flag.Int("workers", 0, "Number of concurrent OCR workers (default: from config or 4)")
	languageFlag := flag.String("language", "", "Language hint for OCR (default: from config or English)")
	configFlag := flag.String("config", "", "Path to YAML config file")
	quietFlag := flag.Bool("quiet", false, "Suppress TUI progress bar and decorative output")
	apiBaseFlag := flag.String("api-base", "", "OpenAI-compatible API base URL override")
	apiKeyFlag := flag.String("api-key", "", "API key override")
	modelFlag := flag.String("model", "", "Model name override")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <input.mkv|input.sup>\n\nFlags:\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		return fmt.Errorf("missing input file argument")
	}

	inputPath := flag.Arg(0)

	// Load config from file
	cfg, err := config.Load(*configFlag)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Environment variable overrides
	if env := os.Getenv("LINOTYPE_API_BASE"); env != "" {
		cfg.API.BaseURL = env
	}
	if env := os.Getenv("LINOTYPE_API_KEY"); env != "" {
		cfg.API.APIKey = env
	}
	if env := os.Getenv("LINOTYPE_MODEL"); env != "" {
		cfg.API.Model = env
	}

	// Track which CLI flags were explicitly provided
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

	// Validate configuration
	if cfg.Format != "ass" && cfg.Format != "webvtt" {
		return fmt.Errorf("unsupported output format %q (must be 'ass' or 'webvtt')", cfg.Format)
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}

	// Set up logger
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:     "15:04:05",
	})
	if *quietFlag {
		logger.SetLevel(log.WarnLevel)
	}

	if !*quietFlag {
		fmt.Fprintf(os.Stderr, "%s — PGS Subtitle OCR Converter\n\n", titleStyle.Render("Linotype"))
	}

	// Check input file existence
	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("input file not found: %w", err)
	}

	// Initialize OCR client
	ocrClient := ocr.NewClient(cfg.API.BaseURL, cfg.API.APIKey, cfg.API.Model)
	systemPrompt := ocr.BuildPrompt(cfg.OCR.Language, cfg.OCR.Prompt)

	// Context with interrupt handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ext := strings.ToLower(filepath.Ext(inputPath))
	switch ext {
	case ".mkv":
		return processMKV(ctx, inputPath, cfg, ocrClient, systemPrompt, *quietFlag, logger)
	case ".sup":
		outPath := getOutputFilename(inputPath, -1, "", cfg.Format)
		return processSUP(ctx, inputPath, outPath, cfg, ocrClient, systemPrompt, *quietFlag, logger)
	default:
		return fmt.Errorf("unsupported input file format %q (expected .mkv or .sup)", ext)
	}
}

// processMKV extracts all PGS tracks from the MKV and converts each.
func processMKV(ctx context.Context, mkvPath string, cfg *config.Config, ocrClient *ocr.Client, prompt string, quiet bool, logger *log.Logger) error {
	logger.Info("Inspecting MKV subtitle tracks", "file", mkvPath)

	tracks, err := extract.ListPGSTracks(ctx, mkvPath)
	if err != nil {
		return fmt.Errorf("failed to list PGS tracks in MKV (ensure ffprobe is on PATH): %w", err)
	}

	if len(tracks) == 0 {
		return fmt.Errorf("no PGS (hdmv_pgs_subtitle) tracks found in %s", mkvPath)
	}

	logger.Info("Found PGS subtitle tracks", "count", len(tracks))
	for _, t := range tracks {
		logger.Info("Track", "index", t.Index, "relative_index", t.RelIndex, "language", t.Language, "title", t.Title)
	}

	for _, track := range tracks {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		tempSup, err := os.CreateTemp("", fmt.Sprintf("linotype_track%d_*.sup", track.RelIndex))
		if err != nil {
			return fmt.Errorf("failed to create temporary .sup file: %w", err)
		}
		tempSupPath := tempSup.Name()
		tempSup.Close()
		defer os.Remove(tempSupPath)

		logger.Info("Extracting PGS track", "track", track.RelIndex, "lang", track.Language)
		if err := extract.ExtractPGS(ctx, mkvPath, track.RelIndex, tempSupPath); err != nil {
			return fmt.Errorf("failed to extract track %d from MKV (ensure ffmpeg is on PATH): %w", track.RelIndex, err)
		}

		outPath := getOutputFilename(mkvPath, track.RelIndex, track.Language, cfg.Format)
		if err := processSUP(ctx, tempSupPath, outPath, cfg, ocrClient, prompt, quiet, logger); err != nil {
			return fmt.Errorf("error processing track %d: %w", track.RelIndex, err)
		}
	}

	return nil
}

// processSUP parses a .sup file, performs OCR, and writes the output subtitle file.
func processSUP(ctx context.Context, supPath, outPath string, cfg *config.Config, ocrClient *ocr.Client, prompt string, quiet bool, logger *log.Logger) error {
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

	totalItems := len(items)
	if totalItems == 0 {
		logger.Warn("No visible subtitle items found in stream")
		return writeOutput(outPath, cfg.Format, nil)
	}

	logger.Info("Extracted subtitle images for OCR", "count", totalItems, "workers", cfg.Workers)

	// Set up concurrency and fail-fast context
	ocrCtx, cancelOCR := context.WithCancel(ctx)
	defer cancelOCR()

	type job struct {
		item pgs.SubtitleItem
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

	// Goroutine to close results when workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Progress tracker
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

	// Assemble subtitle events in chronological order
	events := make([]subtitle.Event, len(items))
	for i, item := range items {
		events[i] = subtitle.Event{
			Index:       item.Index,
			StartTime:   item.StartTime,
			EndTime:     item.EndTime,
			Text:        textByIndex[item.Index],
			X:           item.X,
			Y:           item.Y,
			Width:       item.Width,
			Height:      item.Height,
			VideoWidth:  item.VideoWidth,
			VideoHeight: item.VideoHeight,
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
