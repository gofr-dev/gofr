package logging

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"unsafe"
)

type Config struct {
	Level            Level       `json:"level"`
	Encoding         string      `json:"encoding"`
	OutputPaths      []string    `json:"outputPaths"`
	ErrorOutputPaths []string    `json:"errorOutputPaths"`
	Async            bool        `json:"async"`
	AsyncConfig      AsyncConfig `json:"asyncConfig"`
}

func (cfg Config) Build() (Logger, error) {
	normalOut, err := cfg.buildWriter(cfg.OutputPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to build normal output: %w", err)
	}

	errorOut, err := cfg.buildWriter(cfg.ErrorOutputPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to build error output: %w", err)
	}

	return NewAsyncLogger(cfg.Level, normalOut, errorOut, cfg.AsyncConfig), nil
}

func (cfg Config) buildWriter(paths []string) (io.Writer, error) {
	if len(paths) == 0 {
		return os.Stdout, nil
	}

	writers := make([]io.Writer, 0, len(paths))

	for _, path := range paths {
		switch strings.ToLower(path) {
		case "stdout":
			writers = append(writers, os.Stdout)
		case "stderr":
			writers = append(writers, os.Stderr)
		default:
			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory for %s: %w", path, err)
			}

			f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
			if err != nil {
				return nil, fmt.Errorf("failed to open file %s: %w", path, err)
			}
			writers = append(writers, f)
		}
	}

	if len(writers) == 1 {
		return writers[0], nil
	}

	return io.MultiWriter(writers...), nil
}

// Optimized formatArgs (avoid reflection when possible)
func formatArgs(args []interface{}) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return formatMessage(args[0])
	}

	var b strings.Builder
	for i, arg := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatMessage(arg))
	}
	return b.String()
}

// Optimized message formatting (avoid reflection)
func formatMessage(msg interface{}) string {
	switch v := msg.(type) {
	case string:
		return v
	case []byte:
		return *(*string)(unsafe.Pointer(&v))
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractTraceID looks for trace_id patterns in the log line
func (l *AsyncLogger) extractTraceID(line string) string {
	patterns := []string{"trace_id=", "traceId=", "trace-id=", "traceid="}

	for _, pattern := range patterns {
		if idx := strings.Index(line, pattern); idx >= 0 {
			start := idx + len(pattern)
			end := start

			for end < len(line) && line[end] != ' ' && line[end] != '\t' {
				end++
			}

			if end > start {
				return line[start:end]
			}
		}
	}

	return ""
}

// removeTraceID removes trace ID from the line to avoid duplication
func (l *AsyncLogger) removeTraceID(line, traceID string) string {
	patterns := []string{"trace_id=", "traceId=", "trace-id=", "traceid="}

	for _, pattern := range patterns {
		fullPattern := pattern + traceID
		if idx := strings.Index(line, fullPattern); idx >= 0 {
			before := strings.TrimRight(line[:idx], " \t")
			after := strings.TrimLeft(line[idx+len(fullPattern):], " \t")

			if before != "" && after != "" {
				return before + " " + after
			} else if before != "" {
				return before
			} else if after != "" {
				return after
			}
			return ""
		}
	}

	return line
}

type FieldType uint8

const (
	UnknownType FieldType = iota
	StringType
	IntType
	Int64Type
	Int32Type
	BoolType
	Float64Type
	Float32Type
	DurationType
	TimeType
	ErrorType
	ByteStringType
)

// Field represents a strongly-typed log field
type Field struct {
	Key     string
	Type    FieldType
	Integer int64
	String  string
}

// Typed field constructors for high-performance logging (optional use)
func String(key, value string) Field {
	return Field{Key: key, Type: StringType, String: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Type: IntType, Integer: int64(value)}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Type: Int64Type, Integer: value}
}

func Bool(key string, value bool) Field {
	var i int64
	if value {
		i = 1
	}
	return Field{Key: key, Type: BoolType, Integer: i}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, Type: Float64Type, Integer: int64(math.Float64bits(value))}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Type: DurationType, Integer: int64(value)}
}

func Error(key string, value error) Field {
	return Field{Key: key, Type: ErrorType, String: value.Error()}
}

// Buffer represents a fast, pooled buffer for JSON encoding
type Buffer struct {
	buf []byte
}

func newBuffer() *Buffer {
	return &Buffer{
		buf: make([]byte, 0, defaultBufferSize),
	}
}

func (b *Buffer) Reset() {
	b.buf = b.buf[:0]
}

func (b *Buffer) Len() int {
	return len(b.buf)
}

func (b *Buffer) Cap() int {
	return cap(b.buf)
}

func (b *Buffer) Bytes() []byte {
	return b.buf
}

func (b *Buffer) WriteByte(c byte) {
	b.buf = append(b.buf, c)
}

func (b *Buffer) WriteString(s string) {
	b.buf = append(b.buf, s...)
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *Buffer) AppendInt(i int64) {
	b.buf = strconv.AppendInt(b.buf, i, 10)
}

func (b *Buffer) AppendUint(i uint64) {
	b.buf = strconv.AppendUint(b.buf, i, 10)
}

func (b *Buffer) AppendFloat(f float64, bitSize int) {
	b.buf = strconv.AppendFloat(b.buf, f, 'f', -1, bitSize)
}

func (b *Buffer) AppendTime(t time.Time, layout string) {
	b.buf = t.AppendFormat(b.buf, layout)
}

// Buffer pool for memory management
var bufferPool = sync.Pool{
	New: func() interface{} {
		return newBuffer()
	},
}

func getBuffer() *Buffer {
	return bufferPool.Get().(*Buffer)
}

func putBuffer(buf *Buffer) {
	if buf.Cap() > maxBufferSize {
		return
	}
	buf.Reset()
	bufferPool.Put(buf)
}

// FastJSONEncoder - Zero allocation JSON encoder
type FastJSONEncoder struct {
	buf *Buffer
}

var encoderPool = sync.Pool{
	New: func() interface{} {
		return &FastJSONEncoder{
			buf: getBuffer(),
		}
	},
}

func getEncoder() *FastJSONEncoder {
	return encoderPool.Get().(*FastJSONEncoder)
}

func putEncoder(enc *FastJSONEncoder) {
	putBuffer(enc.buf)
	enc.buf = nil
	encoderPool.Put(enc)
}

// Optimized string escaping
func (enc *FastJSONEncoder) addString(s string) {
	enc.buf.WriteByte('"')

	last := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			continue
		}

		if s[i] >= 0x20 && s[i] != '\\' && s[i] != '"' {
			continue
		}

		enc.buf.WriteString(s[last:i])
		switch s[i] {
		case '\\', '"':
			enc.buf.WriteByte('\\')
			enc.buf.WriteByte(s[i])
		case '\n':
			enc.buf.WriteString("\\n")
		case '\r':
			enc.buf.WriteString("\\r")
		case '\t':
			enc.buf.WriteString("\\t")
		default:
			enc.buf.WriteString("\\u00")
			enc.buf.WriteByte("0123456789abcdef"[s[i]>>4])
			enc.buf.WriteByte("0123456789abcdef"[s[i]&0xF])
		}
		last = i + 1
	}

	enc.buf.WriteString(s[last:])
	enc.buf.WriteByte('"')
}

func (enc *FastJSONEncoder) addKey(key string) {
	if enc.buf.Len() > 1 {
		enc.buf.WriteByte(',')
	}
	enc.addString(key)
	enc.buf.WriteByte(':')
}

// Smart line parsing - extracts structured data from log lines
type LineParser struct {
	fields []Field
}

var lineParserPool = sync.Pool{
	New: func() interface{} {
		return &LineParser{
			fields: make([]Field, 0, 8),
		}
	},
}

func getLineParser() *LineParser {
	parser := lineParserPool.Get().(*LineParser)
	parser.fields = parser.fields[:0]
	return parser
}

func putLineParser(parser *LineParser) {
	for i := range parser.fields {
		parser.fields[i] = Field{}
	}
	parser.fields = parser.fields[:0]

	if cap(parser.fields) > maxFieldSliceSize {
		parser.fields = make([]Field, 0, 8)
	}
	lineParserPool.Put(parser)
}

// parseLogLine intelligently parses different log line formats
func (p *LineParser) parseLogLine(line string) (message string, fields []Field) {
	p.fields = p.fields[:0]

	// Fast path: simple message without key=value pairs
	if !strings.Contains(line, "=") {
		return line, nil
	}

	// Parse key=value pairs from the line
	parts := strings.Fields(line)
	messageBuilder := strings.Builder{}

	for _, part := range parts {
		if idx := strings.Index(part, "="); idx > 0 && idx < len(part)-1 {
			key := part[:idx]
			value := part[idx+1:]

			// Smart type detection and field creation
			field := p.createTypedField(key, value)
			if field.Type != UnknownType {
				p.fields = append(p.fields, field)
				continue
			}
		}

		// Not a key=value pair, add to message
		if messageBuilder.Len() > 0 {
			messageBuilder.WriteByte(' ')
		}
		messageBuilder.WriteString(part)
	}

	message = messageBuilder.String()
	if message == "" && len(p.fields) > 0 {
		message = "structured log entry"
	}

	// Copy fields to avoid pool corruption
	if len(p.fields) > 0 {
		fieldsCopy := make([]Field, len(p.fields))
		copy(fieldsCopy, p.fields)
		return message, fieldsCopy
	}

	return message, nil
}

// createTypedField creates a typed field based on value analysis
func (p *LineParser) createTypedField(key, value string) Field {
	// Remove quotes if present
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
		return Field{Key: key, Type: StringType, String: value}
	}

	// Try integer
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return Field{Key: key, Type: Int64Type, Integer: i}
	}

	// Try float
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return Field{Key: key, Type: Float64Type, Integer: int64(math.Float64bits(f))}
	}

	// Try boolean
	if b, err := strconv.ParseBool(value); err == nil {
		var i int64
		if b {
			i = 1
		}
		return Field{Key: key, Type: BoolType, Integer: i}
	}

	// Try duration
	if strings.HasSuffix(value, "ms") || strings.HasSuffix(value, "s") ||
		strings.HasSuffix(value, "m") || strings.HasSuffix(value, "h") {
		if d, err := time.ParseDuration(value); err == nil {
			return Field{Key: key, Type: DurationType, Integer: int64(d)}
		}
	}

	// Default to string
	return Field{Key: key, Type: StringType, String: value}
}

func (enc *FastJSONEncoder) addField(field Field) {
	enc.addKey(field.Key)

	switch field.Type {
	case StringType:
		enc.addString(field.String)
	case IntType, Int64Type, Int32Type:
		enc.buf.AppendInt(field.Integer)
	case BoolType:
		if field.Integer == 1 {
			enc.buf.WriteString("true")
		} else {
			enc.buf.WriteString("false")
		}
	case Float64Type:
		f := math.Float64frombits(uint64(field.Integer))
		enc.buf.AppendFloat(f, 64)
	case Float32Type:
		f := math.Float32frombits(uint32(field.Integer))
		enc.buf.AppendFloat(float64(f), 32)
	case DurationType:
		d := time.Duration(field.Integer)
		enc.buf.WriteByte('"')
		enc.buf.WriteString(d.String())
		enc.buf.WriteByte('"')
	case TimeType:
		t := time.Unix(0, field.Integer)
		enc.buf.WriteByte('"')
		enc.buf.AppendTime(t, time.RFC3339Nano)
		enc.buf.WriteByte('"')
	case ErrorType:
		enc.addString(field.String)
	case ByteStringType:
		enc.addString(field.String)
	default:
		enc.addString("<unknown>")
	}
}

func (enc *FastJSONEncoder) EncodeLogLine(level Level, timestamp time.Time, line string, traceID string) ([]byte, error) {
	enc.buf.Reset()
	enc.buf.WriteByte('{')

	// Parse the line for structured data
	parser := getLineParser()
	defer putLineParser(parser)

	message, fields := parser.parseLogLine(line)

	// Encode level
	if level >= FATAL {
		enc.addKey("level")
		enc.addString(strings.ToLower(level.String()))
	}

	// Encode time
	if !timestamp.IsZero() {
		enc.addKey("time")
		enc.buf.WriteByte('"')
		enc.buf.AppendTime(timestamp, time.RFC3339Nano)
		enc.buf.WriteByte('"')
	}

	// Encode message
	if message != "" {
		enc.addKey("message")
		enc.addString(message)
	}

	// Encode trace ID
	if traceID != "" {
		enc.addKey("trace_id")
		enc.addString(traceID)
	}

	// Encode extracted fields
	for _, field := range fields {
		enc.addField(field)
	}

	enc.buf.WriteByte('}')
	enc.buf.WriteByte('\n')

	return enc.buf.Bytes(), nil
}
