package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Formatter formats a slog.Record and its attributes into a serialized byte slice.
type Formatter interface {
	Format(r slog.Record, preAttrs []slog.Attr, groups []string) ([]byte, error)
}

// FormatterFunc adapts a function to the Formatter interface.
type FormatterFunc func(r slog.Record, preAttrs []slog.Attr, groups []string) ([]byte, error)

// Format implements Formatter.
func (f FormatterFunc) Format(r slog.Record, preAttrs []slog.Attr, groups []string) ([]byte, error) {
	return f(r, preAttrs, groups)
}

// DefaultTimeFormat is the default timestamp layout ("2006-01-02 15:04:05").
const DefaultTimeFormat = "2006-01-02 15:04:05"

// NewFormatter returns a Formatter based on formatSpec and timeFormat.
//   - "json" or "": outputs a single-line JSON object per record (default; suitable for console, file, or ES/ZincSearch HTTP APIs).
//   - "text": outputs standard key=value text format.
//   - any other string: treated as a custom template supporting placeholders such as
//     {time}, {level}, {msg}, {caller}, {request_id}, {trace_id}, {span_id}, {attrs},
//     or any attribute key {<attr_key>}.
func NewFormatter(formatSpec, timeFormat string) Formatter {
	if strings.TrimSpace(timeFormat) == "" {
		timeFormat = DefaultTimeFormat
	}
	switch strings.ToLower(strings.TrimSpace(formatSpec)) {
	case "", "json":
		return &jsonFormatter{timeFormat: timeFormat}
	case "text":
		return &textFormatter{timeFormat: timeFormat}
	default:
		return &templateFormatter{
			template:   formatSpec,
			timeFormat: timeFormat,
		}
	}
}

func collectAttrs(r slog.Record, preAttrs []slog.Attr, groups []string) []slog.Attr {
	prefix := ""
	if len(groups) > 0 {
		prefix = strings.Join(groups, ".") + "."
	}
	out := make([]slog.Attr, 0, len(preAttrs)+r.NumAttrs())
	var flatten func(p string, a slog.Attr)
	flatten = func(p string, a slog.Attr) {
		a.Value = a.Value.Resolve()
		if a.Equal(slog.Attr{}) {
			return
		}
		if a.Value.Kind() == slog.KindGroup {
			gp := p
			if a.Key != "" {
				gp = p + a.Key + "."
			}
			for _, ga := range a.Value.Group() {
				flatten(gp, ga)
			}
			return
		}
		a.Key = p + a.Key
		out = append(out, a)
	}
	for _, a := range preAttrs {
		flatten(prefix, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		flatten(prefix, a)
		return true
	})
	return out
}

func callerString(pc uintptr) string {
	if pc == 0 {
		return ""
	}
	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()
	if f.File == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
}

type textFormatter struct {
	timeFormat string
}

func (f *textFormatter) Format(r slog.Record, preAttrs []slog.Attr, groups []string) ([]byte, error) {
	var buf bytes.Buffer
	ts := r.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	buf.WriteString("time=")
	buf.WriteString(ts.Format(f.timeFormat))
	buf.WriteString(" level=")
	buf.WriteString(r.Level.String())
	if caller := callerString(r.PC); caller != "" {
		buf.WriteString(" source=")
		buf.WriteString(caller)
	}
	buf.WriteString(" msg=")
	buf.WriteString(quoteIfNeeded(r.Message))

	for _, a := range collectAttrs(r, preAttrs, groups) {
		buf.WriteByte(' ')
		buf.WriteString(a.Key)
		buf.WriteByte('=')
		buf.WriteString(quoteIfNeeded(attrValueString(a.Value)))
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

type jsonFormatter struct {
	timeFormat string
}

func (f *jsonFormatter) Format(r slog.Record, preAttrs []slog.Attr, groups []string) ([]byte, error) {
	ts := r.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	payload := make(map[string]any, 8+len(preAttrs)+r.NumAttrs())
	payload["time"] = ts.Format(f.timeFormat)
	payload["msg"] = r.Message
	if caller := callerString(r.PC); caller != "" {
		payload["source"] = caller
	}
	for _, a := range collectAttrs(r, preAttrs, groups) {
		payload[a.Key] = attrToAny(a.Value)
	}
	var buf bytes.Buffer
	buf.WriteByte('[')
	buf.WriteString(r.Level.String())
	buf.WriteString("] ")
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type templateFormatter struct {
	template   string
	timeFormat string
}

// Format renders a custom log template.
// Built-in placeholders:
//   - {time}: formatted timestamp
//   - {level}: log level (DEBUG, INFO, WARN, ERROR)
//   - {msg}: log message
//   - {caller} or {source}: file:line if add_source is enabled
//   - {<attr_key>}: value of any matching attribute (e.g. {request_id}, {trace_id}, {span_id})
//   - {attrs}: remaining key=value attributes not explicitly referenced in the template
func (f *templateFormatter) Format(r slog.Record, preAttrs []slog.Attr, groups []string) ([]byte, error) {
	ts := r.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	attrs := collectAttrs(r, preAttrs, groups)
	attrMap := make(map[string]string, len(attrs))
	for _, a := range attrs {
		attrMap[a.Key] = attrValueString(a.Value)
	}
	caller := callerString(r.PC)

	usedKeys := make(map[string]bool)
	out := f.template

	replaceToken := func(token, val string) {
		placeholder := "{" + token + "}"
		if strings.Contains(out, placeholder) {
			out = strings.ReplaceAll(out, placeholder, val)
			usedKeys[token] = true
		}
	}

	replaceToken("time", ts.Format(f.timeFormat))
	replaceToken("level", r.Level.String())
	replaceToken("msg", r.Message)
	replaceToken("caller", caller)
	replaceToken("source", caller)

	for k, v := range attrMap {
		replaceToken(k, v)
	}
	// Clear unpopulated well-known trace/request tokens if present in template
	for _, wk := range []string{"request_id", "trace_id", "span_id"} {
		replaceToken(wk, attrMap[wk])
	}

	if strings.Contains(out, "{attrs}") {
		var rem bytes.Buffer
		for _, a := range attrs {
			if usedKeys[a.Key] {
				continue
			}
			if rem.Len() > 0 {
				rem.WriteByte(' ')
			}
			rem.WriteString(a.Key)
			rem.WriteByte('=')
			rem.WriteString(quoteIfNeeded(attrValueString(a.Value)))
		}
		out = strings.ReplaceAll(out, "{attrs}", rem.String())
	}

	out = strings.TrimRight(out, " \t")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out), nil
}

func attrValueString(v slog.Value) string {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'f', -1, 64)
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format(DefaultTimeFormat)
	default:
		return fmt.Sprint(v.Any())
	}
}

func attrToAny(v slog.Value) any {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format(DefaultTimeFormat)
	default:
		if err, ok := v.Any().(error); ok {
			return err.Error()
		}
		return v.Any()
	}
}

func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= ' ' || c == '"' || c == '=' {
			return strconv.Quote(s)
		}
	}
	return s
}
