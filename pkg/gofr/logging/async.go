package logging

import (
	"fmt"
	"golang.org/x/term"
	"io"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Performance constants
const (
	maxPoolObjectSize = 4096
	maxFieldSliceSize = 64
	maxBufferSize     = 64 * 1024 // 64KB
	defaultBufferSize = 1024
	maxFieldsPerEntry = 32
	fileMode          = 0644
)

// LogEntry simplified for line-based logging
type LogEntry struct {
	Level   Level
	Time    time.Time
	Line    string
	TraceID string
	Fields  []Field
}

func (le *LogEntry) Reset() {
	le.Level = INFO
	le.Time = time.Time{}
	le.Line = ""
	le.TraceID = ""
	le.Fields = nil
}

type EnhancedEntryPool struct {
	pool sync.Pool
}

func NewEnhancedEntryPool() *EnhancedEntryPool {
	return &EnhancedEntryPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &LogEntry{}
			},
		},
	}
}

func (p *EnhancedEntryPool) Get() *LogEntry {
	return p.pool.Get().(*LogEntry)
}

func (p *EnhancedEntryPool) Put(entry *LogEntry) {
	entry.Reset()
	p.pool.Put(entry)
}

// LogJob for async processing
type LogJob struct {
	Entry   *LogEntry
	IsError bool
}

type EnhancedJobPool struct {
	pool sync.Pool
}

func NewEnhancedJobPool() *EnhancedJobPool {
	return &EnhancedJobPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &LogJob{}
			},
		},
	}
}

func (jp *EnhancedJobPool) Get() *LogJob {
	return jp.pool.Get().(*LogJob)
}

func (jp *EnhancedJobPool) Put(job *LogJob) {
	job.Entry = nil
	jp.pool.Put(job)
}

// AsyncConfig configures the async logger
type AsyncConfig struct {
	Workers       int
	BufferSize    int
	BatchSize     int
	FlushInterval time.Duration
}

func DefaultAsyncConfig() AsyncConfig {
	return AsyncConfig{
		Workers:       runtime.NumCPU(),
		BufferSize:    1000,
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
	}
}

// AsyncLogger - Line-based high-performance logger
type AsyncLogger struct {
	level     Level
	normalOut io.Writer
	errorOut  io.Writer

	// Terminal detection
	normalIsTerminal bool
	errorIsTerminal  bool

	// Enhanced pools
	entryPool *EnhancedEntryPool
	jobPool   *EnhancedJobPool

	// Worker pool configuration
	workers       int
	bufferSize    int
	batchSize     int
	flushInterval time.Duration

	// Channels for async processing
	logChan  chan *LogJob
	shutdown chan struct{}
	done     chan struct{}

	// Worker management
	wg   sync.WaitGroup
	once sync.Once

	// Stats
	processed int64
	dropped   int64
	mu        sync.RWMutex
	writeMu   sync.Mutex
}

func NewAsyncLogger(level Level, normalOut, errorOut io.Writer, config AsyncConfig) *AsyncLogger {
	l := &AsyncLogger{
		level:            level,
		normalOut:        normalOut,
		errorOut:         errorOut,
		normalIsTerminal: isTerminal(normalOut),
		errorIsTerminal:  isTerminal(errorOut),
		workers:          config.Workers,
		bufferSize:       config.BufferSize,
		batchSize:        config.BatchSize,
		flushInterval:    config.FlushInterval,
		logChan:          make(chan *LogJob, config.BufferSize),
		shutdown:         make(chan struct{}),
		done:             make(chan struct{}),
		entryPool:        NewEnhancedEntryPool(),
		jobPool:          NewEnhancedJobPool(),
	}

	l.startWorkers()
	return l
}

// isTerminal checks if the writer is a terminal
func isTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

// Core logging method - accepts simple strings
func (l *AsyncLogger) log(level Level, line string) {
	if level < l.level {
		return
	}

	// Get objects from pools
	entry := l.entryPool.Get()
	job := l.jobPool.Get()

	// Extract trace ID if present in the line
	traceID := l.extractTraceID(line)
	if traceID != "" {
		line = l.removeTraceID(line, traceID)
	}

	// Populate entry
	entry.Level = level
	entry.Time = time.Now()
	entry.Line = line
	entry.TraceID = traceID

	// Create job
	job.Entry = entry
	job.IsError = level >= ERROR

	// Try to send to worker pool
	select {
	case l.logChan <- job:
		// Success
	default:
		// Channel full, drop the log
		l.entryPool.Put(entry)
		l.jobPool.Put(job)

		l.mu.Lock()
		l.dropped++
		l.mu.Unlock()
	}
}

// Enhanced processJob with zero-allocation encoding
func (l *AsyncLogger) processJob(job *LogJob) {
	defer l.jobPool.Put(job)
	defer l.entryPool.Put(job.Entry)

	var out io.Writer
	var isTerminal bool

	if job.IsError {
		out = l.errorOut
		isTerminal = l.errorIsTerminal
	} else {
		out = l.normalOut
		isTerminal = l.normalIsTerminal
	}

	// Handle terminal pretty printing
	if isTerminal {
		l.prettyPrintToTerminal(job.Entry, out)
	} else {
		// Use fast JSON encoder
		encoder := getEncoder()
		defer putEncoder(encoder)

		data, err := encoder.EncodeLogLine(job.Entry.Level, job.Entry.Time, job.Entry.Line, job.Entry.TraceID)
		if err == nil {
			out.Write(data)
		}
	}

	// Update stats
	l.mu.Lock()
	l.processed++
	l.mu.Unlock()
}

// prettyPrintToTerminal handles terminal output with colors
func (l *AsyncLogger) prettyPrintToTerminal(entry *LogEntry, out io.Writer) {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()

	fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s]",
		entry.Level.color(),
		entry.Level.String()[0:4],
		entry.Time.Format(time.TimeOnly),
	)

	if entry.TraceID != "" {
		fmt.Fprintf(out, " \u001B[38;5;8m%s\u001B[0m", entry.TraceID)
	}

	fmt.Fprintf(out, " %s\n", entry.Line)
}

// Simple logging methods - just pass strings
func (l *AsyncLogger) Debug(args ...interface{}) {
	if l.level <= DEBUG {
		l.log(DEBUG, formatArgs(args))
	}
}

func (l *AsyncLogger) Info(args ...interface{}) {
	if l.level <= INFO {
		l.log(INFO, formatArgs(args))
	}
}

func (l *AsyncLogger) Warn(args ...interface{}) {
	if l.level <= WARN {
		l.log(WARN, formatArgs(args))
	}
}

func (l *AsyncLogger) Error(args ...interface{}) {
	if l.level <= ERROR {
		l.log(ERROR, formatArgs(args))
	}
}

func (l *AsyncLogger) Fatal(args ...interface{}) {
	l.log(FATAL, formatArgs(args))
	l.Close()
	os.Exit(1)
}

// Formatted logging methods
func (l *AsyncLogger) Debugf(format string, args ...interface{}) {
	if l.level <= DEBUG {
		l.log(DEBUG, fmt.Sprintf(format, args...))
	}
}

func (l *AsyncLogger) Infof(format string, args ...interface{}) {
	if l.level <= INFO {
		l.log(INFO, fmt.Sprintf(format, args...))
	}
}

func (l *AsyncLogger) Warnf(format string, args ...interface{}) {
	if l.level <= WARN {
		l.log(WARN, fmt.Sprintf(format, args...))
	}
}

func (l *AsyncLogger) Errorf(format string, args ...interface{}) {
	if l.level <= ERROR {
		l.log(ERROR, fmt.Sprintf(format, args...))
	}
}

func (l *AsyncLogger) Fatalf(format string, args ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, args...))
	l.Close()
	os.Exit(1)
}

// Additional convenience methods
func (l *AsyncLogger) Notice(args ...interface{}) {
	if l.level <= NOTICE {
		l.log(NOTICE, formatArgs(args))
	}
}

func (l *AsyncLogger) Noticef(format string, args ...interface{}) {
	if l.level <= NOTICE {
		l.log(NOTICE, fmt.Sprintf(format, args...))
	}
}

func (l *AsyncLogger) Log(args ...interface{}) {
	l.Info(args...)
}

func (l *AsyncLogger) Logf(format string, args ...interface{}) {
	l.Infof(format, args...)
}

// Worker methods
func (l *AsyncLogger) startWorkers() {
	for i := 0; i < l.workers; i++ {
		l.wg.Add(1)
		go l.worker(i)
	}
}

func (l *AsyncLogger) worker(id int) {
	defer l.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Logger worker %d panicked: %v\n", id, r)
			l.wg.Add(1)
			go l.worker(id)
		}
	}()

	batch := make([]*LogJob, 0, l.batchSize)
	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case job := <-l.logChan:
			batch = append(batch, job)
			if len(batch) >= l.batchSize {
				l.processBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				l.processBatch(batch)
				batch = batch[:0]
			}

		case <-l.shutdown:
			for len(batch) > 0 || len(l.logChan) > 0 {
				select {
				case job := <-l.logChan:
					batch = append(batch, job)
				default:
					if len(batch) > 0 {
						l.processBatch(batch)
						batch = batch[:0]
					}
				}
			}
			return
		}
	}
}

func (l *AsyncLogger) processBatch(jobs []*LogJob) {
	for _, job := range jobs {
		l.processJob(job)
	}
}

func (l *AsyncLogger) Close() error {
	l.once.Do(func() {
		close(l.shutdown)
		l.wg.Wait()
		close(l.done)
	})
	return nil
}

func (l *AsyncLogger) Stats() (processed, dropped int64) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.processed, l.dropped
}

func (l *AsyncLogger) ChangeLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func NewLogger(level Level) (Logger, error) {
	config := DefaultAsyncConfig()
	config.Workers = runtime.NumCPU() * 2
	config.BufferSize = 2000
	return NewAsyncLogger(level, os.Stdout, os.Stderr, config), nil
}
