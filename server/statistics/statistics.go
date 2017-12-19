package statistics

import (
	"sync"

        "github.com/ubports/ubuntu-push/logger"

)

type Statistics struct {
        updateLock sync.Mutex
	logger logger.Logger

        //registered devices per...
        //5 mins
        reg_devices_5min int32
        //60 mins
        reg_devices_60min int32
        //1 day
        reg_devices_1day int32
        //7 days
        reg_devices_7day int32

        unicasts_5min int32
        unicasts_60min int32
        unicasts_1day int32
        unicasts_7day int32

}

func NewStatistics(logger logger.Logger) *Statistics {

	return &Statistics{
		logger: logger,
		updateLock: sync.Mutex{},
	}

}

func (stats Statistics) Reset() {

	stats.reg_devices_5min = 0
	stats.reg_devices_60min = 0
	stats.reg_devices_1day = 0
	stats.reg_devices_7day = 0
	stats.unicasts_5min = 0
	stats.unicasts_60min = 0
	stats.unicasts_1day = 0
	stats.unicasts_7day = 0

}

//Shall be called periodically (every 5 secs) to output stats
func (stats Statistics) PrintStats() {

	stats.logger.Infof("Test")

}

