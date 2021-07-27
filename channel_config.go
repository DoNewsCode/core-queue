package queue

// ChannelConfig describes the key name JobFrom each queue, also known as channel.
type ChannelConfig struct {
	Delayed  string
	Failed   string
	Reserved string
	Waiting  string
	Timeout  string
}
