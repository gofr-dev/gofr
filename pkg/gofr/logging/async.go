package logging

import (
	"encoding/json"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"runtime"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/version"
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

// LogEntry - Updated to match sync logger behavior
type LogEntry struct {
	Level       Level       `json:"level"`
	Time        time.Time   `json:"time"`
	Message     interface{} `json:"message"` // ← Changed from 'Line string'
	TraceID     string      `json:"trace_id,omitempty"`
	GofrVersion string      `json:"gofrVersion"` // ← Added this field
	Fields      []Field     // Keep for advanced structured logging
}

func (le *LogEntry) Reset() {
	le.Level = INFO
	le.Time = time.Time{}
	le.Message = nil // ← Updated
	le.TraceID = ""
	le.GofrVersion = "" // ← Added this
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
	Workers          int
	BufferSize       int
	BatchSize        int
	OutputPaths      []string // e.g. []string{"stdout", "logs/app.log"}
	ErrorOutputPaths []string // e.g. []string{"stderr", "logs/error.log"}
	FlushInterval    time.Duration
}

func DefaultAsyncConfig() AsyncConfig {
	return AsyncConfig{
		Workers:       runtime.NumCPU(),
		BufferSize:    1000,
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
	}
}

// AsyncLogger - High-performance async logger matching sync logger behavior
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

func NewFileLogger(filePath string, level Level) Logger {
	config := Config{
		Level:            level,
		Encoding:         "json",
		OutputPaths:      []string{filePath},
		ErrorOutputPaths: []string{filePath},
		Async:            true,
		AsyncConfig:      DefaultAsyncConfig(),
	}

	logger, err := config.Build()
	if err != nil {
		// Fallback to stdout/stderr if file fails
		fallbackConfig := Config{
			Level:            level,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
			Async:            true,
			AsyncConfig:      DefaultAsyncConfig(),
		}
		logger, _ = fallbackConfig.Build()
	}

	return logger
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

// extractTraceIDAndFilterArgs - exact copy from sync logger
func (l *AsyncLogger) extractTraceIDAndFilterArgs(args []interface{}) (traceID string, filtered []interface{}) {
	filtered = make([]interface{}, 0, len(args))

	for _, arg := range args {
		if m, ok := arg.(map[string]interface{}); ok {
			if tid, exists := m["__trace_id__"].(string); exists && traceID == "" {
				traceID = tid
				continue
			}
		}

		filtered = append(filtered, arg)
	}

	return traceID, filtered
}

// Updated core logging method - matches sync logger behavior
func (l *AsyncLogger) logf(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	// Get objects from pools
	entry := l.entryPool.Get()
	job := l.jobPool.Get()

	// Extract trace ID and filter args (like sync logger)
	traceID, filteredArgs := l.extractTraceIDAndFilterArgs(args)

	// Populate entry (matching sync logger logic)
	entry.Level = level
	entry.Time = time.Now()
	entry.TraceID = traceID
	entry.GofrVersion = version.Framework

	// Message handling logic (exactly like sync logger)
	switch {
	case len(filteredArgs) == 1 && format == "":
		entry.Message = filteredArgs[0]
	case len(filteredArgs) != 1 && format == "":
		entry.Message = filteredArgs
	case format != "":
		entry.Message = fmt.Sprintf(format, filteredArgs...)
	}

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
		// Use standard JSON encoder for non-terminal output
		_ = json.NewEncoder(out).Encode(job.Entry)
	}

	// Update stats
	l.mu.Lock()
	l.processed++
	l.mu.Unlock()
}

// prettyPrintToTerminal - matches sync logger behavior exactly
func (l *AsyncLogger) prettyPrintToTerminal(entry *LogEntry, out io.Writer) {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()

	// Exact same format as sync logger
	fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s]",
		entry.Level.color(),
		entry.Level.String()[0:4],
		entry.Time.Format(time.TimeOnly))

	if entry.TraceID != "" {
		fmt.Fprintf(out, " \u001B[38;5;8m%s\u001B[0m", entry.TraceID)
	}

	fmt.Fprint(out, " ")

	// Pretty printing logic - exactly like sync logger
	if fn, ok := entry.Message.(PrettyPrint); ok {
		fn.PrettyPrint(out)
	} else {
		fmt.Fprintf(out, "%v\n", entry.Message)
	}
}

// Simple logging methods - updated to match sync logger
func (l *AsyncLogger) Debug(args ...interface{}) {
	l.logf(DEBUG, "", args...)
}

func (l *AsyncLogger) Info(args ...interface{}) {
	l.logf(INFO, "", args...)
}

func (l *AsyncLogger) Warn(args ...interface{}) {
	l.logf(WARN, "", args...)
}

func (l *AsyncLogger) Error(args ...interface{}) {
	l.logf(ERROR, "", args...)
}

func (l *AsyncLogger) Fatal(args ...interface{}) {
	l.logf(FATAL, "", args...)
	l.Close()
	os.Exit(1)
}

// Formatted logging methods
func (l *AsyncLogger) Debugf(format string, args ...interface{}) {
	l.logf(DEBUG, format, args...)
}

func (l *AsyncLogger) Infof(format string, args ...interface{}) {
	l.logf(INFO, format, args...)
}

func (l *AsyncLogger) Warnf(format string, args ...interface{}) {
	l.logf(WARN, format, args...)
}

func (l *AsyncLogger) Errorf(format string, args ...interface{}) {
	l.logf(ERROR, format, args...)
}

func (l *AsyncLogger) Fatalf(format string, args ...interface{}) {
	l.logf(FATAL, format, args...)
	l.Close()
	os.Exit(1)
}

// Additional methods to match interface
func (l *AsyncLogger) Notice(args ...interface{}) {
	l.logf(NOTICE, "", args...)
}

func (l *AsyncLogger) Noticef(format string, args ...interface{}) {
	l.logf(NOTICE, format, args...)
}

func (l *AsyncLogger) Log(args ...interface{}) {
	l.logf(INFO, "", args...)
}

func (l *AsyncLogger) Logf(format string, args ...interface{}) {
	l.logf(INFO, format, args...)
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
