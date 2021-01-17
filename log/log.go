package log

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stdout)
}

func Infof(format string, v ...interface{}) {
	log.Infof(format, v)
}

func Infoln(args ...interface{}) {
	log.Infoln(args)
}

func Warnf(format string, v ...interface{}) {
	log.Warnf(format, v)
}

func Warnln(args ...interface{}) {
	log.Warnln(args)
}

func Errorf(format string, v ...interface{}) {
	log.Errorf(format, v)
}

func Errorln(args ...interface{}) {
	log.Errorln(args)
}

func Debugf(format string, v ...interface{}) {
	log.Debugf(format, v)
}

func Debugln(args ...interface{}) {
	log.Debugln(args)
}

func Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v)
}

func Fatalln(args ...interface{}) {
	log.Fatalln(args)
}

func SetLevel(newLevel uint) {
	log.SetLevel(log.Level(newLevel))
}
