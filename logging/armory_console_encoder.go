/*
 * Copyright 2022 Armory, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

// NewArmoryDevConsoleEncoder encoder that produces colored human-readable output
// Not suitable for production use, see: zapcore.MapObjectEncoder comments.
func NewArmoryDevConsoleEncoder(disableColor bool) zapcore.Encoder {
	if disableColor {
		color.NoColor = true
	} else {
		// Forces color on for the dev cluster, because by default the color lib, disables color when there is no tty.
		color.NoColor = false
	}
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
var faint = color.New(color.Faint)

func (c *consoleEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	out := bufferpool.Get()

	out.AppendString(faint.Sprintf(ent.Time.Local().Format("01/02 03:04:05pm")))
	out.AppendString(tab)

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
	out.AppendString(lC.Sprintf(ent.Level.CapitalString()))

	if ent.LoggerName != "" {
		out.AppendString(tab)
		out.AppendByte('[')
		out.AppendString(ent.LoggerName)
		out.AppendByte(']')
	}

	var src string
	if c.m.Fields["src"] != nil {
		src = c.m.Fields["src"].(string)
	}
	if ent.Caller.Defined || src != "" {
		out.AppendString(tab)
		if src != "" {
			out.AppendString(color.New(color.FgBlue, color.Bold, color.Underline).Sprintf(src))
		} else {
			out.AppendString(color.New(color.FgBlue, color.Bold, color.Underline).Sprintf(ent.Caller.TrimmedPath()))
		}
	}

	mC := color.New()
	switch ent.Level {
	case zapcore.DebugLevel:
		mC = color.New(color.FgHiGreen)
		break
	case zapcore.WarnLevel:
		mC = color.New(color.FgHiYellow)
		break
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		mC = color.New(color.FgHiRed)
		break
	}
	out.AppendString(tab)
	out.AppendString(mC.Sprintf(ent.Message))

	stack, err := c.writeContext(out, fields)
	if stack != "" {
		ent.Stack = stack
	}

	if err != "" {
		out.AppendByte(newline)
		out.AppendString("Contributing Error: ")
		out.AppendString(color.New(color.FgHiRed).Sprintf(err))
	}

	if strings.TrimSpace(ent.Stack) != "" {
		out.AppendByte(newline)
		out.AppendString(
			reg.ReplaceAllString(ent.Stack,
				fmt.Sprintf("${1}%s", color.BlueString(color.New(color.FgBlue, color.Bold, color.Underline).Sprintf("${2}${3}"))),
			),
		)
	}
	out.AppendByte(newline)

	return out, nil
}

func (c *consoleEncoder) writeContext(line *buffer.Buffer, fields []zapcore.Field) (string, string) {
	clone := c.Clone().(*consoleEncoder)
	for _, field := range fields {
		field.AddTo(clone)
	}

	// filter out empty values
	fieldsToLog := lo.PickBy(clone.m.Fields, func(_ string, value any) bool {
		return !reflect.ValueOf(&value).Elem().IsZero()
	})

	var stack string
	if fieldsToLog["stacktrace"] != nil {
		stack = fieldsToLog["stacktrace"].(string)
		delete(fieldsToLog, "stacktrace")
	}

	// delete some redundant fields
	delete(fieldsToLog, "stack")
	delete(fieldsToLog, "src")

	var err string
	if fieldsToLog["error"] != nil {
		err = fieldsToLog["error"].(string)
		delete(fieldsToLog, "error")
	}
	if fieldsToLog["errorVerbose"] != nil {
		err = fieldsToLog["errorVerbose"].(string)
		delete(fieldsToLog, "errorVerbose")
	}

	if len(fieldsToLog) == 0 {
		return stack, err
	}

	line.AppendString(tab)
	line.AppendString("[ ")
	keys := maps.Keys(fieldsToLog)
	sort.Strings(keys)
	for i, key := range keys {
		value := fieldsToLog[key]
		line.AppendString(color.New(color.FgHiBlue).Sprintf(key))
		line.AppendString(": ")
		line.AppendByte('\'')
		line.AppendString(color.YellowString(fmt.Sprintf("%v", value)))
		line.AppendByte('\'')
		if i+1 == len(keys) {
			line.AppendString(" ")
		} else {
			line.AppendString(", ")
		}
	}
	line.AppendByte(']')

	return stack, err
}
