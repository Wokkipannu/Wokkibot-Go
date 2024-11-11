package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"wokkibot/utils"
	"wokkibot/wokkibot"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var downloadCommand = discord.SlashCommandCreate{
	Name:        "download",
	Description: "Download a video",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "url",
			Description: "The URL of the video",
			Required:    true,
		},
	},
}

type DownloadTask struct {
	e                 *handler.CommandEvent
	url               string
	filePathProcessed string
	tempDir           string
	maxFileSize       int
}

type DownloadProgress struct {
	ProgressPercentage string `json:"progress_percentage"`
}

const (
	downloadTimeout   = 3 * time.Minute
	conversionTimeout = 5 * time.Minute
	updateInterval    = 1 * time.Second
	defaultBitrate    = "1M"
)

var taskQueue = make(chan DownloadTask, 10)
var once sync.Once

func HandleDownload(b *wokkibot.Wokkibot) handler.CommandHandler {
	once.Do(func() {
		go downloadWorker()
	})

	return func(e *handler.CommandEvent) error {
		url := e.SlashCommandInteractionData().String("url")

		if url == "" {
			return e.CreateMessage(discord.NewMessageCreateBuilder().SetContent("No URL provided").Build())
		}

		if err := e.Respond(discord.InteractionResponseTypeDeferredCreateMessage, nil); err != nil {
			return err
		}

		if err := os.MkdirAll("downloads", 0755); err != nil {
			return err
		}
		tempDir, err := os.MkdirTemp("downloads", "video_*")
		if err != nil {
			return err
		}

		guild, _ := e.Guild()

		// Temporary fix for specific site
		if strings.HasPrefix(url, "https://ylilauta.org/file/") {
			parts := strings.Split(url, "/")
			if len(parts) == 0 {
				handleError(e, "Invalid URL format", "Invalid URL format")
				return fmt.Errorf("invalid URL format")
			}

			fileID := parts[len(parts)-1]

			if len(fileID) < 4 {
				handleError(e, "File ID is too short", "File ID is too short")
				return fmt.Errorf("file ID is too short")
			}

			subPath := fmt.Sprintf("%s/%s", fileID[:2], fileID[2:4])

			newURL := fmt.Sprintf("https://i.ylilauta.org/%s/%s-apple.mp4", subPath, fileID)

			url = newURL
		}
		// Also convert direct links to the -apple version
		if strings.HasPrefix(url, "https://i.ylilauta.org/") {
			parts := strings.Split(url, "/")
			if len(parts) == 0 {
				handleError(e, "Invalid URL format", "Invalid URL format")
				return fmt.Errorf("invalid URL format")
			}

			filename := parts[len(parts)-1]
			if !strings.HasSuffix(filename, "-apple.mp4") {
				filename = strings.TrimSuffix(filename, ".mp4") + "-apple.mp4"

				parts[len(parts)-1] = filename
				newURL := strings.Join(parts, "/")

				url = newURL
			}
		}

		task := DownloadTask{
			e:                 e,
			url:               url,
			filePathProcessed: filepath.Join(tempDir, fmt.Sprintf("%s_processed.mp4", utils.GenerateRandomName(10))),
			tempDir:           tempDir,
			maxFileSize:       calculateMaximumFileSizeForGuild(guild),
		}
		taskQueue <- task

		_, err = e.UpdateInteractionResponse(discord.NewMessageUpdateBuilder().
			SetContent("Waiting for previous download tasks to finish...").
			Build())
		return err
	}
}

func downloadWorker() {
	for task := range taskQueue {
		handleDownloadAndConversion(task)
	}
}

func handleDownloadAndConversion(task DownloadTask) {
	e := task.e

	defer cleanup(task.tempDir)

	e.UpdateInteractionResponse(discord.NewMessageUpdateBuilder().
		SetContent("Starting video download...").
		Build())

	downloadedFile, err := downloadVideo(task, e)
	if err != nil {
		handleError(e, "Error while downloading video", err.Error())
		return
	}

	processedFile, err := convertVideo(task, e, downloadedFile)
	if err != nil {
		handleError(e, "Error while converting video", err.Error())
		return
	}

	if err := attachFile(e, processedFile); err != nil {
		handleError(e, "Error while attaching file", err.Error())
		return
	}
}

func downloadVideo(task DownloadTask, e *handler.CommandEvent) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	output := filepath.Join(task.tempDir, "video_download.%(ext)s")
	cmd := exec.CommandContext(ctx, "yt-dlp",
		task.url,
		"-o", output,
		"--max-filesize", fmt.Sprintf("%dM", task.maxFileSize),
		"--format-sort", "res:720,codec:h264",
		"--merge-output-format", "mp4",
		"--progress-template", "{\"progress_percentage\": \"%(progress._percent_str)s\"}",
		"--newline",
	)
	return executeWithProgress(e, task, cmd, ctx, "download", "")
}

func convertVideo(task DownloadTask, e *handler.CommandEvent, downloadedFile string) (string, error) {
	codec, err := getVideoCodec(downloadedFile)
	if err != nil {
		return "", err
	}

	if codec == "h264" {
		return downloadedFile, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), conversionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", downloadedFile,
		"-c:v", "h264",
		"-b:v", defaultBitrate,
		"-c:a", "aac",
		"-pix_fmt", "yuv420p",
		"-f", "mp4",
		task.filePathProcessed,
		"-progress", "pipe:1",
		"-nostats",
	)

	return executeWithProgress(e, task, cmd, ctx, "conversion", downloadedFile)
}

func executeWithProgress(e *handler.CommandEvent, task DownloadTask, cmd *exec.Cmd, ctx context.Context, operation string, downloadedFile string) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("error getting stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("error starting command: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	lastUpdate := time.Now()
	var lastPercentage float64

	e.UpdateInteractionResponse(discord.NewMessageUpdateBuilder().
		SetContent(fmt.Sprintf("Starting video %s\n%s %.2f%%", operation, createProgressBar(0.0), 0.0)).
		Build())

	for scanner.Scan() {
		if operation == "download" {
			if strings.Contains(scanner.Text(), "File is larger than max-filesize") {
				_ = cmd.Process.Kill()
				return "", fmt.Errorf("file size exceeds the maximum allowed size for this guild. Maximum is %dMB", task.maxFileSize)
			}

			if !strings.HasPrefix(scanner.Text(), "{") || !json.Valid([]byte(scanner.Text())) {
				continue
			}

			var progress DownloadProgress
			if err := json.Unmarshal([]byte(scanner.Text()), &progress); err != nil {
				continue
			}

			percentage, _ := strconv.ParseFloat(strings.TrimSuffix(progress.ProgressPercentage, "%"), 64)
			if time.Since(lastUpdate) >= updateInterval && percentage != lastPercentage {
				progress := createProgressBar(percentage)

				e.UpdateInteractionResponse(discord.NewMessageUpdateBuilder().
					SetContent(fmt.Sprintf("Downloading video\n%s %.2f%%", progress, percentage)).
					Build())

				lastUpdate = time.Now()
				lastPercentage = percentage
			}
		}

		if operation == "conversion" {
			totalDuration, _ := getVideoDuration(downloadedFile)

			line := scanner.Text()

			if strings.Contains(line, "out_time=") {
				timeIndex := strings.Index(line, "out_time=")
				if timeIndex != -1 {
					currentTime := line[timeIndex+9:]

					var progressTime float64
					parts := strings.Split(currentTime, ":")
					if len(parts) == 3 {
						hours, _ := strconv.ParseFloat(parts[0], 64)
						minutes, _ := strconv.ParseFloat(parts[1], 64)
						seconds, _ := strconv.ParseFloat(parts[2], 64)
						progressTime = hours*3600 + minutes*60 + seconds
					}

					if totalDuration > 0 {
						progressPercentage := (progressTime / totalDuration) * 100
						if time.Since(lastUpdate) >= 1*time.Second && progressPercentage != lastPercentage {
							progress := createProgressBar(progressPercentage)
							e.UpdateInteractionResponse(discord.NewMessageUpdateBuilder().
								SetContent(fmt.Sprintf("Converting video\n%s %.2f%%", progress, progressPercentage)).
								Build())
							lastUpdate = time.Now()
							lastPercentage = progressPercentage
						}
					}
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			handleError(e, "Timed out", fmt.Sprintf("%s canceled as it took too long", utils.CapitalizeFirstLetter(operation)))
			return "", fmt.Errorf("operation timed out")
		}
		return "", fmt.Errorf("command failed: %w", err)
	}

	if operation == "download" {
		files, err := filepath.Glob(filepath.Join(task.tempDir, "video_download.*"))
		if err != nil {
			return "", fmt.Errorf("error finding downloaded file: %w", err)
		}

		return files[0], nil
	}

	return task.filePathProcessed, nil
}

func getVideoCodec(videoFile string) (string, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoFile,
	)

	ouptut, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting video codec: %w", err)
	}

	return strings.TrimSpace(string(ouptut)), nil
}

func createProgressBar(percentage float64) string {
	filledBlocks := int(percentage / 100 * float64(20))

	bar := strings.Repeat("█", filledBlocks) + strings.Repeat("░", 20-filledBlocks)

	return bar
}

func cleanup(tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		fmt.Printf("Error while removing downloaded files: %v", err)
	}
}

func getVideoDuration(videoFile string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoFile)

	durationOutput, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("error getting duration: %w", err)
	}

	totalDuration, err := strconv.ParseFloat(strings.TrimSpace(string(durationOutput)), 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing duration: %w", err)
	}

	return totalDuration, nil
}

func attachFile(e *handler.CommandEvent, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		handleError(e, "Error while opening file", err.Error())
		return err
	}
	defer file.Close()

	_, err = e.UpdateInteractionResponse(discord.NewMessageUpdateBuilder().
		SetContent("").
		AddFile(filePath, filePath, file).
		Build())
	if err != nil {
		return err
	}

	return nil
}

func handleError(e *handler.CommandEvent, message string, err string) {
	e.UpdateInteractionResponse(discord.NewMessageUpdateBuilder().
		SetEmbeds(discord.NewEmbedBuilder().
			SetTitlef(message).
			SetDescription(err).
			SetColor(utils.RGBToInteger(255, 0, 0)).
			Build()).
		SetContent("").
		Build())
}

func calculateMaximumFileSizeForGuild(guild discord.Guild) int {
	if guild.PremiumTier == discord.PremiumTier2 {
		return 50
	} else if guild.PremiumTier == discord.PremiumTier3 {
		return 100
	} else {
		return 10
	}
}
