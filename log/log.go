package log

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stdout)
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func Infoln(msg string, v ...interface{}) {
	if len(v) == 0 {
		log.Infoln(msg)
		return
	}

	log.Infof(msg, v)
}

func Warnln(msg string, v ...interface{}) {
	if len(v) == 0 {
		log.Warnln(msg)
		return
	}

	log.Warnf(msg, v)
}

func Errorln(msg string, v ...interface{}) {
	if len(v) == 0 {
		log.Errorln(msg)
		return
	}

	log.Errorf(msg, v)
}

func Debugln(msg string, v ...interface{}) {
	if len(v) == 0 {
		log.Debugln(msg)
		return
	}

	log.Debugf(msg, v)
}

func Fatalln(msg string, v ...interface{}) {
	if len(v) == 0 {
		log.Fatalln(msg)
		return
	}

	log.Fatalf(msg, v...)
}

func SetLevel(newLevel uint) {
	log.SetLevel(log.Level(newLevel))
}
