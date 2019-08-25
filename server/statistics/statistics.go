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
		val60min: ring.New(12), //12 * 5 min interval = 1 hour
		val1day: ring.New(288), //288 * 5 min interval = 1 day
		val7day: ring.New(2016), //2016 * 5 min interval = 7 days
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

func (statsvalue *StatsValue) Reset5min() {
	statsvalue.val5min = 0
}

func (statsvalue *StatsValue) Report() (int32, int32, int32, int32) {

	var val60min int32 = 0
	statsvalue.val60min.Do(func(p interface{}) {
		if p != nil {
			val60min += p.(int32)
		}
	})
	val60min = val60min / 12
	var val1day int32 = 0
	statsvalue.val1day.Do(func(p interface{}) {
		if p != nil {
			val1day += p.(int32)
		}
	})
	val1day = val1day / 288
	var val7day int32 = 0
	statsvalue.val7day.Do(func(p interface{}) {
		if p != nil {
			val7day += p.(int32)
		}
	})
	val7day = val7day / 2016
	return statsvalue.val5min, val60min, val1day, val7day
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

	//Device-specific accumulation
	devices_specific map[string]*StatsValue

	//Channel-specific accumulation
	channel_specific map[string]*StatsValue

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
	//Enable the following line for testing statistics gathering and aggregation
	//go result.TestStats()
	return result
}

func (stats *Statistics) Accumulate() {
	stats.devices_online.Accumulate()
	stats.unicasts_total.Accumulate()
	stats.broadcasts_total.Accumulate()
	for _, value := range stats.devices_specific {
		value.Accumulate()
	}
	for _, value := range stats.channel_specific {
		value.Accumulate()
	}
}

func (stats *Statistics) Reset5min() {
	stats.unicasts_total.Reset5min()
	stats.broadcasts_total.Reset5min()
}

func (stats *Statistics) DecreaseDevices(device_name string, channel_name string) {
	stats.updating.Lock()
	stats.devices_online.val5min--
	if(stats.devices_specific == nil) {
		stats.devices_specific = make(map[string]*StatsValue)
	}
	if(stats.devices_specific[device_name] == nil) {
		stats.devices_specific[device_name] = NewStatsValue()
	}
	stats.devices_specific[device_name].val5min--
	if(stats.channel_specific == nil) {
		stats.channel_specific = make(map[string]*StatsValue)
	}
	if(stats.channel_specific[channel_name] == nil) {
		stats.channel_specific[channel_name] = NewStatsValue()
	}
	stats.channel_specific[channel_name].val5min--

	stats.updating.Unlock()
}

func (stats *Statistics) IncreaseDevices(device_name string, channel_name string) {
	stats.updating.Lock()
	stats.devices_online.val5min++
    if(stats.devices_specific == nil) {
	stats.devices_specific = make(map[string]*StatsValue)
    }
	if(stats.devices_specific[device_name] == nil) {
		stats.devices_specific[device_name] = NewStatsValue()
	}
	stats.devices_specific[device_name].val5min++
    if(stats.channel_specific == nil) {
	stats.channel_specific = make(map[string]*StatsValue)
	}
	if(stats.channel_specific[channel_name] == nil) {
		stats.channel_specific[channel_name] = NewStatsValue()
	}
    stats.channel_specific[channel_name].val5min++
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
		stats.IncreaseDevices("bacon", "16.04/rc")
		<-t.C
	}
}

//Shall be called periodically (every 5 minutes) to output stats
func (stats *Statistics) PrintStats() {
	t := time.NewTicker(time.Minute * 5)
	for {
		stats.updating.Lock()
		stats.Accumulate()

		//Tally the accumulated values
		devices_online_5min, devices_online_60min, devices_online_1day, devices_online_7day := stats.devices_online.Report()
		unicasts_total_5min, unicasts_total_60min, unicasts_total_1day, unicasts_total_7day := stats.unicasts_total.Report()
		broadcasts_total_5min, broadcasts_total_60min, broadcasts_total_1day, broadcasts_total_7day := stats.broadcasts_total.Report()

		stats.logger.Infof("Usage statistics:")
		stats.logger.Infof("        |  Devices   |  Unicasts  |  Broadcasts |")
		stats.logger.Infof("5 mins  | %10v | %10v | %10v |", devices_online_5min, unicasts_total_5min, broadcasts_total_5min)
		stats.logger.Infof("60 mins | %10v | %10v | %10v |", devices_online_60min, unicasts_total_60min, broadcasts_total_60min)
		stats.logger.Infof("1 day   | %10v | %10v | %10v |", devices_online_1day, unicasts_total_1day, broadcasts_total_1day)
		stats.logger.Infof("7 days  | %10v | %10v | %10v |", devices_online_7day, unicasts_total_7day, broadcasts_total_7day)
		stats.logger.Infof("")
		stats.logger.Infof("Device statistics:")
		stats.logger.Infof("%15v | %10v | %10v | %10v | %10v", "Device", "5 mins","60 mins","1 day","7 days")
		for key, value := range stats.devices_specific {
			devices_online_5min, devices_online_60min, devices_online_1day, devices_online_7day = value.Report()
		stats.logger.Infof("%15v | %10v | %10v | %10v | %10v", key, devices_online_5min, devices_online_60min, devices_online_1day, devices_online_7day)
		}
		stats.logger.Infof("")
		stats.logger.Infof("Channel statistics:")
		stats.logger.Infof("%30v | %10v | %10v | %10v | %10v", "Channel", "5 mins","60 mins","1 day","7 days")
		for key, value := range stats.channel_specific {
			devices_online_5min, devices_online_60min, devices_online_1day, devices_online_7day = value.Report()
		stats.logger.Infof("%30v | %10v | %10v | %10v | %10v", key, devices_online_5min, devices_online_60min, devices_online_1day, devices_online_7day)
		}
		stats.Reset5min()
		stats.updating.Unlock()

		//Wait until timer has elapsed
		<-t.C
	}
}
