package main

import (
    "fmt"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "os"
    "strings"
    "time"
)

func InitLogger(level string) {
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

    output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
    output.FormatLevel = func(i interface{}) string {
        const (
            colorBlack = iota + 30
            colorRed
            colorGreen
            colorYellow
            colorBlue
        )
        var colorMap = map[string]int{
            "debug": colorYellow,
            "info":  colorGreen,
            "warn":  colorBlue,
            "error": colorRed,
        }
        colorize := func(s string, c int) string {
            return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
        }
        level, _ := i.(string)
        return colorize(strings.ToUpper(level), colorMap[level])

    }
    output.FormatFieldName = func(i interface{}) string {
        return fmt.Sprintf("%s:", i)
    }
    output.FormatFieldValue = func(i interface{}) string {
        return strings.ToUpper(fmt.Sprintf("%s", i))
    }
    log.Logger = log.Output(output)
    zerolog.SetGlobalLevel(getLevel(level))
    zerolog.TimestampFunc = func() time.Time {
        return time.Now().UTC()
    }
}

func getLevel(level string) zerolog.Level {
    switch strings.ToLower(level) {
    case "debug":
        return zerolog.DebugLevel
    case "info":
        return zerolog.InfoLevel
    case "warning":
        return zerolog.WarnLevel
    case "error":
        return zerolog.ErrorLevel
    default:
        return zerolog.InfoLevel
    }
}
