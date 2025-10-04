package ui

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "os"
    "os/signal"
    "strings"
    "sync"
    "syscall"
    "time"

    "github.com/pterm/pterm"
)

// Suggestion represents the data to be presented to the user.
// It decouples the UI from the internal LLM suggestion format.
type Suggestion struct {
	Explanation string
	Command     string
	Title       string // e.g., "AI Suggestion" or "Generated Command"
}

// Presenter handles the standardized display of suggestions and user interaction.
type Presenter struct {
    spinner     *pterm.SpinnerPrinter
    startTime   time.Time
    mu          sync.Mutex
    timerCancel context.CancelFunc
    timerWG     sync.WaitGroup
    ttyWriter   io.WriteCloser // 用於spinner輸出到/dev/tty,繞過stderr重定向
}

// NewPresenter creates a new Presenter.
func NewPresenter() *Presenter {
	return &Presenter{}
}

// Render displays a suggestion and handles user input.
// Returns the user's new prompt, whether to proceed, and any error.
func (p *Presenter) Render(suggestion Suggestion) (string, bool, error) {
    pterm.DefaultHeader.Println(suggestion.Title)

	if suggestion.Explanation != "" {
		pterm.Println(pterm.Red("Explanation:"))
		pterm.Println(suggestion.Explanation)
		pterm.Println()
	}

	pterm.Println(pterm.Green("Suggested Command:"))
	pterm.Println(pterm.LightGreen(suggestion.Command))
	pterm.Println()

	pterm.Println("Options:")
	pterm.Println(pterm.LightWhite("  [Enter] - Execute the suggested command"))
	pterm.Println(pterm.LightWhite("  [n/no]  - Reject and exit"))
	pterm.Println(pterm.LightWhite("  [other] - Provide a new prompt for a different suggestion"))
	pterm.Println()
	pterm.Print("Select an option: ")

    // 支援 Ctrl+C 即時取消，不阻塞在輸入讀取
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    reader := bufio.NewReader(os.Stdin)
    readCh := make(chan string, 1)
    errCh := make(chan error, 1)

    go func(c context.Context) {
        line, err := reader.ReadString('\n')
        if err != nil {
            select {
            case <-c.Done():
                return
            default:
                select { case errCh <- err: default: }
                return
            }
        }
        select {
        case <-c.Done():
            return
        default:
            select { case readCh <- line: default: }
        }
    }(ctx)

    var input string
    select {
    case <-ctx.Done():
        pterm.Warning.Println("Operation cancelled by user.")
        return "", false, nil
    case err := <-errCh:
        return "", false, fmt.Errorf("error reading user input: %w", err)
    case line := <-readCh:
        input = strings.TrimSpace(strings.ToLower(line))
    }

	switch input {
	case "": // Enter
		return "", true, nil
	case "n", "no":
		pterm.Warning.Println("Operation cancelled by user.")
		return "", false, nil
	default:
		return input, true, nil
	}
}

// ShowLoading displays a spinner with a message.
func (p *Presenter) ShowLoading(message string) {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Stop any running timer goroutine and spinner to avoid overlapping output
    if p.timerCancel != nil {
        p.timerCancel()
        p.timerWG.Wait()
        p.timerCancel = nil
    }
    if p.spinner != nil {
        // Silently stop old spinner to avoid SUCCESS/FAIL markers
        _ = p.spinner.Stop()
        p.spinner = nil
    }

    p.spinner, _ = pterm.DefaultSpinner.Start(message)
}

// StopLoading stops the spinner.
func (p *Presenter) StopLoading(success bool) {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Cancel timer goroutine first to avoid concurrent updates and duplicated text
    if p.timerCancel != nil {
        p.timerCancel()
        p.timerWG.Wait()
        p.timerCancel = nil
    }

    if p.spinner != nil {
        if success {
            // 顯示成功訊息(帶✓標記)
            p.spinner.Success()
            // 短暫延遲讓用戶看到成功訊息
            time.Sleep(300 * time.Millisecond)
            // 清除成功訊息行,避免累積
            // 需要直接輸出到tty以繞過stderr重定向
            if p.ttyWriter != nil {
                fmt.Fprint(p.ttyWriter, "\r\033[K")
            } else {
                fmt.Print("\r\033[K")
            }
        } else {
            // 失敗時直接清除,不顯示失敗標記
            _ = p.spinner.Stop()
            if p.ttyWriter != nil {
                fmt.Fprint(p.ttyWriter, "\r\033[K")
            } else {
                fmt.Print("\r\033[K")
            }
        }
        p.spinner = nil
    }

    // Close tty writer after spinner is done
    if p.ttyWriter != nil {
        p.ttyWriter.Close()
        p.ttyWriter = nil
    }
}

// ShowLoadingWithTimer displays a spinner with a message and time counter.
func (p *Presenter) ShowLoadingWithTimer(baseMessage string) error {
    p.mu.Lock()

    // If a timer is already running, cancel and wait it to exit to avoid races
    if p.timerCancel != nil {
        p.timerCancel()
        p.timerWG.Wait()
        p.timerCancel = nil
    }
    // If a spinner exists, silently stop it to avoid stacked output
    if p.spinner != nil {
        p.spinner.Stop()
        p.spinner = nil
    }
    // Close previous tty writer if exists
    if p.ttyWriter != nil {
        p.ttyWriter.Close()
        p.ttyWriter = nil
    }

    p.startTime = time.Now()

    // Open /dev/tty for spinner output to bypass stderr redirection in shell hooks
    // This ensures spinner is always visible even when stderr is redirected to /dev/null
    tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
    var spinnerWriter io.Writer = os.Stderr // fallback to stderr if tty unavailable
    if err == nil {
        p.ttyWriter = tty
        spinnerWriter = tty
    }

    // Create a new spinner with custom sequence and DISABLE built-in timer.
    // We render the (Xs) timer ourselves. If we don't disable the built-in
    // pterm timer, the UI will show duplicated time like "(1s) (1s)".
    // RemoveWhenDone ensures the spinner text is cleared when stopped.
    spinner := *pterm.DefaultSpinner.
        WithShowTimer(false).
        WithRemoveWhenDone(true).
        WithWriter(spinnerWriter)
    spinner.Sequence = []string{"▀", "▄", "█", "▐", "▌", "▀", "▄", "█"}

    // Start spinner with base message only; timer text will be updated by our goroutine
    sp, err := spinner.Start(fmt.Sprintf("%s...", baseMessage))
    if err != nil {
        // If spinner fails to start, clean up tty writer and return error
        if p.ttyWriter != nil {
            p.ttyWriter.Close()
            p.ttyWriter = nil
        }
        p.mu.Unlock()
        return fmt.Errorf("failed to start spinner: %w", err)
    }
    p.spinner = sp

    // Create a cancelable context for the timer goroutine
    ctx, cancel := context.WithCancel(context.Background())
    p.timerCancel = cancel

    // Copy required state into the closure to avoid using shared fields directly
    start := p.startTime
    p.timerWG.Add(1)
    p.mu.Unlock()

    // Start timer goroutine with proper error handling
    go func(spinnerPtr *pterm.SpinnerPrinter, startAt time.Time, label string, ctx context.Context) {
        defer p.timerWG.Done()

        // Ensure ticker is always stopped to prevent goroutine leak
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()

        lastSec := -1
        for {
            select {
            case <-ctx.Done():
                // Context cancelled, exit cleanly
                return
            case <-ticker.C:
                // Update text only when seconds change to avoid redundant redraws
                elapsedSec := int(time.Since(startAt).Seconds())
                if elapsedSec != lastSec {
                    // Use non-blocking update to prevent deadlock
                    spinnerPtr.UpdateText(fmt.Sprintf("%s... (%ds)", label, elapsedSec))
                    lastSec = elapsedSec
                }
            }
        }
    }(sp, start, baseMessage, ctx)

    return nil
}

// ShowErrorTriggersList displays only the current captured error type
func (p *Presenter) ShowErrorTriggersList(
	currentError string,
	enabledTriggers []string,
) {
	pterm.Println()
	pterm.Info.Printf("Error captured: [%s]\n", currentError)
	pterm.Println()
}
