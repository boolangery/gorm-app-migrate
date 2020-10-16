package apps

import (
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

func init() {
	logger.SetLevel(logrus.InfoLevel)
}

func SetLogLevel(level logrus.Level) {
	logger.SetLevel(level)
}

func SetLogger(l *logrus.Logger) {
	logger = l
}
