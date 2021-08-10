// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/pingcap/tidb/util/logutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	. "github.com/pingcap/check"
	"github.com/pingcap/errors"
)

func TestLog(t *testing.T) {
	TestingT(t)
}

type testLogSuite struct{}

var _ = Suite(&testLogSuite{})

func (s *testLogSuite) TestTestLogger(c *C) {
	logger, buffer := makeTestLogger()
	logger.Warn("the message", zap.Int("number", 123456), zap.Ints("array", []int{7, 8, 9}))
	c.Assert(
		buffer.Stripped(), Equals,
		`{"$lvl":"WARN","$msg":"the message","number":123456,"array":[7,8,9]}`,
	)
	buffer.Reset()
	logger.ErrorFilterContextCanceled("the message", zap.Int("number", 123456),
		zap.Ints("array", []int{7, 8, 9}), zap.Error(context.Canceled))
	c.Assert(buffer.Stripped(), Equals, "")
	buffer.Reset()
	logger.ErrorFilterContextCanceled("the message", zap.Int("number", 123456),
		zap.Ints("array", []int{7, 8, 9}), ShortError(errors.Annotate(context.Canceled, "extra info")))
	c.Assert(buffer.Stripped(), Equals, "")
}

// makeTestLogger creates a Logger instance which produces JSON logs.
func makeTestLogger() (Logger, *zaptest.Buffer) {
	buffer := new(zaptest.Buffer)
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			LevelKey:       "$lvl",
			MessageKey:     "$msg",
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
		}),
		buffer,
		zap.DebugLevel,
	))
	return Logger{Logger: logger}, buffer
}

func (s *testLogSuite) TestLogLevel(c *C) {
	logLevel := "warning"
	cfg := &Config{
		Level: logLevel,
	}
	cfg.Adjust()

	c.Assert(InitLogger(cfg), IsNil)
	c.Assert(Props().Level.String(), Equals, zap.WarnLevel.String())
	c.Assert(L().Check(zap.InfoLevel, "This is an info log"), IsNil)
	c.Assert(L().Check(zap.ErrorLevel, "This is an error log"), NotNil)

	SetLevel(zap.InfoLevel)
	c.Assert(Props().Level.String(), Equals, zap.InfoLevel.String())
	c.Assert(L().Check(zap.WarnLevel, "This is a warn log"), NotNil)
	c.Assert(L().Check(zap.DebugLevel, "This is a debug log"), IsNil)
}

func captureStdout(f func()) ([]string, error) {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w

	f()

	var buf bytes.Buffer
	output := make(chan string, 1)
	errs := make(chan error, 1)

	go func() {
		_, err := io.Copy(&buf, r)
		output <- buf.String()
		errs <- err
		r.Close()
	}()

	os.Stdout = stdout
	w.Close()
	return strings.Split(<-output, "\n"), <-errs
}

func (s *testLogSuite) TestInitSlowQueryLogger(c *C) {
	logLevel := "debug"
	cfg := &Config{
		Level: logLevel,
	}
	cfg.Adjust()

	output, err := captureStdout(func() {
		c.Assert(InitLogger(cfg), IsNil)
		logutil.SlowQueryLogger.Debug("this is test info")
	})
	c.Assert(err, IsNil)
	c.Assert(len(output), Equals, 2)
	c.Assert(output[0], Matches, ".*component.*slow query logger.*")
}
