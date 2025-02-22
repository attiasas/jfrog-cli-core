package progressbar

import (
	"github.com/gookit/color"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"golang.org/x/term"
	golangLog "log"
	"math"
	"os"
	"sync"
	"time"
)

const (
	ProgressBarWidth     = 20
	longProgressBarWidth = 100
	ProgressRefreshRate  = 200 * time.Millisecond
)

type Color int64

const (
	WHITE Color = iota
	GREEN       = 1
)

type ProgressBarMng struct {
	// A container of all external mpb bar objects to be displayed.
	container *mpb.Progress
	// A synchronization lock object.
	barsRWMutex sync.RWMutex
	// A wait group for all progress bars.
	barsWg *sync.WaitGroup
	// The log file
	logFile *os.File
}

func NewBarsMng() (mng *ProgressBarMng, shouldInit bool, err error) {
	// Determine whether the progress bar should be displayed or not
	shouldInit, err = ShouldInitProgressBar()
	if !shouldInit || err != nil {
		return
	}
	mng = &ProgressBarMng{}
	// Init log file
	mng.logFile, err = corelog.CreateLogFile()
	if err != nil {
		return
	}
	log.Info("Log path:", mng.logFile.Name())
	log.SetLogger(log.NewLoggerWithFlags(corelog.GetCliLogLevel(), mng.logFile, golangLog.Ldate|golangLog.Ltime|golangLog.Lmsgprefix))

	mng.barsWg = new(sync.WaitGroup)
	mng.container = mpb.New(
		mpb.WithOutput(os.Stderr),
		mpb.WithWidth(longProgressBarWidth),
		mpb.WithWaitGroup(mng.barsWg),
		mpb.WithRefreshRate(ProgressRefreshRate))
	return
}

func (bm *ProgressBarMng) NewTasksWithHeadlineProg(totalTasks int64, headline string, spinner bool, color Color, taskType string) *TasksWithHeadlineProg {
	bm.barsWg.Add(1)
	prog := TasksWithHeadlineProg{}
	if spinner {
		prog.headlineBar = bm.NewHeadlineBarWithSpinner(headline)
	} else {
		prog.headlineBar = bm.NewHeadlineBar(headline)
	}

	// If totalTasks is 0 - phase is already finished in previous run.
	if totalTasks == 0 {
		prog.tasksProgressBar = bm.NewDoneTasksProgressBar()
	} else {
		prog.tasksProgressBar = bm.NewTasksProgressBar(totalTasks, color, taskType)
	}
	prog.emptyLine = bm.NewHeadlineBar("")
	return &prog
}

func (bm *ProgressBarMng) QuitTasksWithHeadlineProg(prog *TasksWithHeadlineProg) {
	prog.headlineBar.Abort(true)
	prog.headlineBar = nil
	prog.tasksProgressBar.bar.Abort(true)
	prog.tasksProgressBar = nil
	prog.emptyLine.Abort(true)
	prog.emptyLine = nil
	bm.barsWg.Done()
}

// NewHeadlineBar Initializes a new progress bar for headline, with an optional spinner
func (bm *ProgressBarMng) NewHeadlineBarWithSpinner(msg string) *mpb.Bar {
	return bm.container.New(1,
		mpb.SpinnerStyle("∙∙∙∙∙∙", "●∙∙∙∙∙", "∙●∙∙∙∙", "∙∙●∙∙∙", "∙∙∙●∙∙", "∙∙∙∙●∙", "∙∙∙∙∙●", "∙∙∙∙∙∙").PositionLeft(),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(msg),
		),
	)
}

func (bm *ProgressBarMng) NewHeadlineBar(msg string) *mpb.Bar {
	return bm.container.Add(1,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(msg),
		),
	)
}

// Increment increments completed tasks count by 1.
func (bm *ProgressBarMng) Increment(prog *TasksWithHeadlineProg) {
	bm.barsRWMutex.RLock()
	defer bm.barsRWMutex.RUnlock()
	prog.tasksProgressBar.bar.Increment()
	prog.tasksProgressBar.tasksCount++
}

// Increment increments completed tasks count by n.
func (bm *ProgressBarMng) IncBy(n int, prog *TasksWithHeadlineProg) {
	bm.barsRWMutex.RLock()
	defer bm.barsRWMutex.RUnlock()
	prog.tasksProgressBar.bar.IncrBy(n)
	prog.tasksProgressBar.tasksCount += int64(n)
}

// DoneTask increase tasks counter to the number of totalTasks.
func (bm *ProgressBarMng) DoneTask(prog *TasksWithHeadlineProg) {
	bm.barsRWMutex.RLock()
	defer bm.barsRWMutex.RUnlock()
	diff := prog.tasksProgressBar.total - prog.tasksProgressBar.tasksCount
	// Handle large number of total tasks
	for ; diff > math.MaxInt; diff -= math.MaxInt {
		prog.tasksProgressBar.bar.IncrBy(math.MaxInt)
	}
	prog.tasksProgressBar.bar.IncrBy(int(diff))
}

func (bm *ProgressBarMng) NewTasksProgressBar(totalTasks int64, color Color, taskType string) *TasksProgressBar {
	pb := &TasksProgressBar{}
	filter := filterColor(color)
	if taskType == "" {
		taskType = "Tasks"
	}
	pb.bar = bm.container.New(0,
		mpb.BarStyle().Lbound("|").Filler(filter).Tip(filter).Padding("⬛").Refiller("").Rbound("|"),
		mpb.BarRemoveOnComplete(),
		mpb.AppendDecorators(
			decor.Name(" "+taskType+": "),
			decor.CountersNoUnit("%d/%d"),
		),
	)
	pb.IncGeneralProgressTotalBy(totalTasks)
	return pb
}

func (bm *ProgressBarMng) NewCounterProgressBar(headline string, num int64, valColor color.Color) *TasksProgressBar {
	pb := &TasksProgressBar{}
	pb.bar = bm.container.Add(num,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(headline),
			decor.Any(func(statistics decor.Statistics) string {
				return valColor.Render(pb.GetTotal())
			}),
		),
	)
	return pb
}

func (bm *ProgressBarMng) NewDoneTasksProgressBar() *TasksProgressBar {
	pb := &TasksProgressBar{}
	pb.bar = bm.container.Add(1,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name("Done ✅"),
		),
	)
	return pb
}

func (bm *ProgressBarMng) NewStringProgressBar(headline string, updateFn func() string) *TasksProgressBar {
	pb := &TasksProgressBar{}
	pb.bar = bm.container.Add(1,
		nil,
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(headline),
			decor.Any(func(statistics decor.Statistics) string {
				return updateFn()
			}),
		),
	)
	return pb
}

func (bm *ProgressBarMng) GetBarsWg() *sync.WaitGroup {
	return bm.barsWg
}

func (bm *ProgressBarMng) GetLogFile() *os.File {
	return bm.logFile
}

func filterColor(color Color) (filter string) {
	switch color {
	case GREEN:
		filter = "🟩"
	case WHITE:
		filter = "⬜"
	default:
		filter = "⬜"
	}
	return
}

// The ShouldInitProgressBar func is used to determine whether the progress bar should be displayed.
// This default implementation will init the progress bar if the following conditions are met:
// CI == false (or unset) and Stderr is a terminal.
var ShouldInitProgressBar = func() (bool, error) {
	ci, err := utils.GetBoolEnvValue(coreutils.CI, false)
	if ci || err != nil {
		return false, err
	}
	if !log.IsStdErrTerminal() {
		return false, err
	}
	err = setTerminalWidthVar()
	if err != nil {
		return false, err
	}
	return true, nil
}

var terminalWidth int

// Get terminal dimensions
func setTerminalWidthVar() error {
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		return err
	}
	// -5 to avoid edges
	terminalWidth = width - 5
	if terminalWidth <= 0 {
		terminalWidth = 5
	}
	return err
}
