package statistics

import (
	"sync"
	"time"

        "github.com/ubports/ubuntu-push/logger"

)

type Statistics struct {

	logger logger.Logger
	//Mutex to protect the statistics from concurrent updates
        updating sync.Mutex

        //registered devices per...
        //5 mins
        reg_devices_5min int32
        //60 mins
        reg_devices_60min int32
        //1 day
        reg_devices_1day int32
        //7 days
        reg_devices_7day int32

        unicasts_5min uint32
        unicasts_60min uint32
        unicasts_1day uint32
        unicasts_7day uint32

        broadcasts_5min uint32
        broadcasts_60min uint32
        broadcasts_1day uint32
        broadcasts_7day uint32

}

func NewStatistics(logger logger.Logger) *Statistics {

	return &Statistics{
		logger: logger,
		updating: sync.Mutex{},
	}

}

func (stats *Statistics) ResetAll() {

	stats.Reset5min()
	stats.Reset60min()
	stats.Reset1day()
	stats.Reset7day()
}

func (stats *Statistics) Reset5min() {

	stats.reg_devices_5min = 0
	stats.unicasts_5min = 0
}

func (stats *Statistics) Reset60min() {

	stats.reg_devices_60min = 0
	stats.unicasts_60min = 0
}

func (stats *Statistics) Reset1day() {

	stats.reg_devices_1day = 0
	stats.unicasts_1day = 0
}

func (stats *Statistics) Reset7day() {

	stats.reg_devices_7day = 0
	stats.unicasts_7day = 0
}

func (stats *Statistics) DecreaseDevices() {

	stats.updating.Lock()
	stats.reg_devices_5min--
	stats.reg_devices_60min--
	stats.reg_devices_1day--
	stats.reg_devices_7day--
	stats.updating.Unlock()
}

func (stats *Statistics) IncreaseDevices() {

	stats.updating.Lock()
	stats.reg_devices_5min++
	stats.reg_devices_60min++
	stats.reg_devices_1day++
	stats.reg_devices_7day++
	stats.updating.Unlock()
}

func (stats *Statistics) IncreaseUnicasts() {

	stats.updating.Lock()
	stats.unicasts_5min++
	stats.unicasts_60min++
	stats.unicasts_1day++
	stats.unicasts_7day++
	stats.updating.Unlock()
}

//Shall be called periodically (every 5 m,inutes) to output stats
func (stats *Statistics) PrintStats() {

	callcount := 0
	t := time.NewTicker(time.Minute * 5)
	for {
		stats.updating.Lock()
		stats.logger.Infof("")
		stats.logger.Infof("        |  Devices   |  Unicasts  | Broadcasts |")
		stats.logger.Infof("5 mins  | %10v | %10v | %10v |", stats.reg_devices_5min, stats.unicasts_5min, stats.broadcasts_5min)
		stats.logger.Infof("60 mins | %10v | %10v | %10v |", stats.reg_devices_60min, stats.unicasts_60min, stats.broadcasts_60min)
		stats.logger.Infof("1 day   | %10v | %10v | %10v |", stats.reg_devices_1day, stats.unicasts_1day, stats.broadcasts_1day)
		stats.logger.Infof("7 days  | %10v | %10v | %10v |", stats.reg_devices_7day, stats.unicasts_7day, stats.broadcasts_7day)
		callcount++
		stats.Reset5min()
		if callcount % 12 == 0 {
			stats.Reset60min()
		}
		if callcount % 288 == 0 {
			stats.Reset1day()
		}
		if callcount % 2016 == 0 {
			stats.Reset7day()
		}
		stats.updating.Unlock()
		<-t.C
	}
}

