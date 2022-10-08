package logging

import (
	"fmt"
	"github.com/armory-io/go-commons/bufferpool"
	"github.com/fatih/color"
	"github.com/samber/lo"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/maps"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

// NewArmoryDevConsoleEncoder encoder that produces colored human readable output
// Not suitable for production use, see: zapcore.MapObjectEncoder comments.
func NewArmoryDevConsoleEncoder() zapcore.Encoder {
	return &consoleEncoder{
		m: zapcore.NewMapObjectEncoder(),
	}
}

type consoleEncoder struct {
	m *zapcore.MapObjectEncoder
}

func (c *consoleEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	return c.m.AddArray(key, marshaler)
}

func (c *consoleEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	return c.m.AddObject(key, marshaler)
}

func (c *consoleEncoder) AddBinary(key string, value []byte) {
	c.m.AddBinary(key, value)
}

func (c *consoleEncoder) AddByteString(key string, value []byte) {
	c.m.AddByteString(key, value)
}

func (c *consoleEncoder) AddBool(key string, value bool) {
	c.m.AddBool(key, value)
}

func (c *consoleEncoder) AddComplex128(key string, value complex128) {
	c.m.AddComplex128(key, value)
}

func (c *consoleEncoder) AddComplex64(key string, value complex64) {
	c.m.AddComplex64(key, value)
}

func (c *consoleEncoder) AddDuration(key string, value time.Duration) {
	c.m.AddDuration(key, value)
}

func (c *consoleEncoder) AddFloat64(key string, value float64) {
	c.m.AddFloat64(key, value)
}

func (c *consoleEncoder) AddFloat32(key string, value float32) {
	c.m.AddFloat32(key, value)
}

func (c *consoleEncoder) AddInt(key string, value int) {
	c.m.AddInt(key, value)
}

func (c *consoleEncoder) AddInt64(key string, value int64) {
	c.m.AddInt64(key, value)
}

func (c *consoleEncoder) AddInt32(key string, value int32) {
	c.m.AddInt32(key, value)
}

func (c *consoleEncoder) AddInt16(key string, value int16) {
	c.m.AddInt16(key, value)
}

func (c *consoleEncoder) AddInt8(key string, value int8) {
	c.m.AddInt8(key, value)
}

func (c *consoleEncoder) AddString(key, value string) {
	c.m.AddString(key, value)
}

func (c *consoleEncoder) AddTime(key string, value time.Time) {
	c.m.AddTime(key, value)
}

func (c *consoleEncoder) AddUint(key string, value uint) {
	c.m.AddUint(key, value)
}

func (c *consoleEncoder) AddUint64(key string, value uint64) {
	c.m.AddUint64(key, value)
}

func (c *consoleEncoder) AddUint32(key string, value uint32) {
	c.m.AddUint32(key, value)
}

func (c *consoleEncoder) AddUint16(key string, value uint16) {
	c.m.AddUint16(key, value)
}

func (c *consoleEncoder) AddUint8(key string, value uint8) {
	c.m.AddUint8(key, value)
}

func (c *consoleEncoder) AddUintptr(key string, value uintptr) {
	c.m.AddUintptr(key, value)
}

func (c *consoleEncoder) AddReflected(key string, value interface{}) error {
	return c.m.AddReflected(key, value)
}

func (c *consoleEncoder) OpenNamespace(key string) {
	c.m.OpenNamespace(key)
}

func (c *consoleEncoder) Clone() zapcore.Encoder {
	m := zapcore.NewMapObjectEncoder()
	for k, v := range c.m.Fields {
		_ = m.AddReflected(k, v)
	}
	return &consoleEncoder{
		m: m,
	}
}

const (
	tab     = "  "
	newline = '\n'
)

var reg = regexp.MustCompile(`(\s+)(.*)(/.*?\.go:\d+)`)

func (c *consoleEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	line := bufferpool.Get()

	lC := color.New(color.FgHiWhite)
	switch ent.Level {
	case zapcore.DebugLevel:
		lC = color.New(color.FgHiGreen, color.Bold)
		break
	case zapcore.InfoLevel:
		lC = color.New(color.FgHiCyan, color.Bold)
		break
	case zapcore.WarnLevel:
		lC = color.New(color.FgHiYellow, color.Bold)
		break
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		lC = color.New(color.FgHiRed, color.Bold)
		break
	}
	line.AppendByte(newline)
	line.AppendString(color.YellowString("lvl: "))
	line.AppendString(lC.Sprintf(ent.Level.CapitalString()))

	line.AppendByte(newline)
	line.AppendString(color.YellowString("msg: "))
	line.AppendString(ent.Message)

	src, stack, err := c.writeContext(line, fields)
	if stack != "" {
		ent.Stack = stack
	}

	line.AppendByte(newline)
	line.AppendString(color.YellowString("dtm: "))
	line.AppendString(ent.Time.Local().Format("Jan 02 2006 03:04:05.00pm"))

	if ent.Caller.Defined || src != "" {
		line.AppendByte(newline)
		line.AppendString(color.YellowString("src: "))
		if src != "" {
			line.AppendString(color.New(color.FgBlue, color.Bold, color.Underline).Sprintf(src))
		} else {
			line.AppendString(color.New(color.FgBlue, color.Bold, color.Underline).Sprintf(ent.Caller.FullPath()))
		}
		line.AppendString(tab)
	}

	if err != "" {
		line.AppendByte(newline)
		line.AppendString(color.YellowString("err: "))
		line.AppendString(color.New(color.FgHiRed, color.Bold).Sprintf(err))
	}

	if ent.LoggerName != "" {
		line.AppendByte(newline)
		line.AppendString(color.YellowString("lgr: "))
		line.AppendString(ent.LoggerName)
		line.AppendString(tab)
	}

	if strings.TrimSpace(ent.Stack) != "" {
		line.AppendByte(newline)
		line.AppendString(color.YellowString("stracktrace: "))
		line.AppendByte(newline)
		line.AppendString(
			reg.ReplaceAllString(ent.Stack,
				fmt.Sprintf("${1}%s", color.BlueString(color.New(color.FgBlue, color.Bold, color.Underline).Sprintf("${2}${3}"))),
			),
		)
	}

	line.AppendByte(newline)

	return line, nil
}

func (c *consoleEncoder) writeContext(line *buffer.Buffer, fields []zapcore.Field) (string, string, string) {
	clone := c.Clone().(*consoleEncoder)
	for _, field := range fields {
		field.AddTo(clone)
	}

	// filter out empty values
	fieldsToLog := lo.PickBy(clone.m.Fields, func(_ string, value any) bool {
		return !reflect.ValueOf(&value).Elem().IsZero()
	})

	var src string
	if fieldsToLog["src"] != nil {
		src = fieldsToLog["src"].(string)
	}
	delete(fieldsToLog, "src")

	var stack string
	if fieldsToLog["stacktrace"] != nil {
		stack = fieldsToLog["stacktrace"].(string)
	}
	delete(fieldsToLog, "stacktrace")

	var err string
	if fieldsToLog["error"] != nil {
		err = fieldsToLog["error"].(string)
	}
	delete(fieldsToLog, "error")

	if len(fieldsToLog) == 0 {
		return src, stack, err
	}

	line.AppendByte(newline)
	line.AppendString(color.YellowString("ctx: "))
	line.AppendByte('[')
	line.AppendByte(newline)
	keys := maps.Keys(fieldsToLog)
	sort.Strings(keys)
	for _, key := range keys {
		value := fieldsToLog[key]
		line.AppendString(tab)
		line.AppendString(color.New(color.FgHiBlue).Sprintf(key))
		line.AppendString(": ")
		line.AppendString(color.YellowString(fmt.Sprintf("%v", value)))
		line.AppendByte(newline)
	}
	line.AppendByte(']')

	return src, stack, err
}
