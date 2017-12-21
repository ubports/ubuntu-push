package statistics

import (
	"sync"
	"time"
	"container/ring"
	
	"github.com/ubports/ubuntu-push/logger"
)

type StatsValue struct {
	val5min int32
	val60min *ring.Ring
	val1day *ring.Ring
	val7day *ring.Ring
}

func NewStatsValue() *StatsValue {
	result := &StatsValue {
	}
	result.val60min = ring.New(12)
	for i := 0; i < result.val60min.Len(); i++ {
		result.val60min.Value = int32(0)
		result.val60min = result.val60min.Next()
	}
	result.val1day = ring.New(288)
	for i := 0; i < result.val1day.Len(); i++ {
		result.val1day.Value = int32(0)
		result.val1day = result.val1day.Next()
	}
	result.val7day = ring.New(2016)
	for i := 0; i < result.val7day.Len(); i++ {
		result.val7day.Value = int32(0)
		result.val7day = result.val7day.Next()
	}

	return result
}

func (statsvalue *StatsValue) Accumulate() {
	statsvalue.val60min.Value = statsvalue.val5min
	statsvalue.val60min = statsvalue.val60min.Next()
	statsvalue.val1day.Value = statsvalue.val5min
	statsvalue.val1day = statsvalue.val1day.Next()
	statsvalue.val7day.Value = statsvalue.val5min
	statsvalue.val7day = statsvalue.val7day.Next()	
}

func (statsvalue *StatsValue) Report() (int32, int32, int32, int32) {
	
	var val60min int32 = 0
	statsvalue.val60min.Do(func(p interface{}) {
			val60min += p.(int32)
		})
	var val1day int32 = 0
	statsvalue.val1day.Do(func(p interface{}) {
			val1day += p.(int32)
		})
	var val7day int32 = 0
	statsvalue.val7day.Do(func(p interface{}) {
			val7day += p.(int32)
		})	
	return statsvalue.val5min, val60min / 12, val1day / 288, val7day / 2016
}

type Statistics struct {
	logger logger.Logger
	//Mutex to protect the statistics from concurrent updates
	updating sync.Mutex

	//Devices currently online
	devices_online *StatsValue
	
	//Total unicasts sent
	unicasts_total *StatsValue
	
	//Total broadcasts sent
	broadcasts_total *StatsValue
}

func NewStatistics(logger logger.Logger) *Statistics {
	result := &Statistics{
		logger: logger,
		updating: sync.Mutex{},
		devices_online: NewStatsValue(),
		unicasts_total: NewStatsValue(),
		broadcasts_total: NewStatsValue(),
	}
	go result.PrintStats()
	//go result.TestStats()
	return result
}

func (stats *Statistics) Accumulate() {
	stats.devices_online.Accumulate()
	stats.unicasts_total.Accumulate()
	stats.broadcasts_total.Accumulate()
}

func (stats *Statistics) DecreaseDevices() {
	stats.updating.Lock()
	stats.devices_online.val5min--
	stats.updating.Unlock()
}

func (stats *Statistics) IncreaseDevices() {
	stats.updating.Lock()
	stats.devices_online.val5min++
	stats.updating.Unlock()
}

func (stats *Statistics) IncreaseUnicasts() {
	stats.updating.Lock()
	stats.unicasts_total.val5min++
	stats.updating.Unlock()
}

func (stats *Statistics) IncreaseBroadcasts() {
	stats.updating.Lock()
	stats.broadcasts_total.val5min++
	stats.updating.Unlock()
}

func (stats *Statistics) TestStats() {
		t := time.NewTicker(time.Millisecond * 500)
	for {
		stats.devices_online.val5min = 20
		stats.IncreaseUnicasts()
		stats.IncreaseBroadcasts()
		<-t.C	
	}
}

//Shall be called periodically (every 5 m,inutes) to output stats
func (stats *Statistics) PrintStats() {
	callcount := 0
	t := time.NewTicker(time.Minute * 5)
	for {
		stats.updating.Lock()
		stats.Accumulate()
		
		//Tally the accumulated values
		devices_online_5min, devices_online_60min, devices_online_1day, devices_online_7day := stats.devices_online.Report()
		unicasts_total_5min, unicasts_total_60min, unicasts_total_1day, unicasts_total_7day := stats.unicasts_total.Report()
		broadcasts_total_5min, broadcasts_total_60min, broadcasts_total_1day, broadcasts_total_7day := stats.broadcasts_total.Report()
		
		stats.logger.Infof("Current usage statistics:")
		stats.logger.Infof("        |  Devices   |  Unicasts  | Broadcasts |")
		stats.logger.Infof("5 mins  | %10v | %10v | %10v |", devices_online_5min, unicasts_total_5min, broadcasts_total_5min)
		stats.logger.Infof("60 mins | %10v | %10v | %10v |", devices_online_60min, unicasts_total_60min, broadcasts_total_60min)
		stats.logger.Infof("1 day   | %10v | %10v | %10v |", devices_online_1day, unicasts_total_1day, broadcasts_total_1day)
		stats.logger.Infof("7 days  | %10v | %10v | %10v |", devices_online_7day, unicasts_total_7day, broadcasts_total_7day)
		stats.updating.Unlock()
		
		//Prevent overflow
		callcount++
		if callcount == 4294967295 {
			callcount = 0
		}
		
		//Wait until timer has elapsed
		<-t.C
	}
}
