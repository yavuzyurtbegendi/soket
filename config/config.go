package config

import "time"

type Config struct {
	WritePeriod      time.Duration
	PongPeriod       time.Duration
	PingPeriod       time.Duration
	MaxMessageSize   int
	MessageQueueSize int
}

type ConfigParam func(*Config)

func initConfig() *Config {
	return &Config{
		WritePeriod:      10 * time.Second,
		PingPeriod:       30 * time.Second,
		PongPeriod:       90 * time.Second,
		MaxMessageSize:   512,
		MessageQueueSize: 100,
	}
}

func LoadConfig(configs []ConfigParam) *Config {
	config := initConfig()
	for _, cnf := range configs {
		cnf(config)
	}
	return config
}

// Messages to be sent are queued in a channel
// channel queue size tells channel how many message should we keep
func WithMessageQueueSize(messageQueueSize int) ConfigParam {
	return func(c *Config) {
		if messageQueueSize < 5 {
			panic("messageQueueSize cannot be lower than 5")
		}
		c.MessageQueueSize = messageQueueSize
	}
}

// How long writing to socket should wait?
// err := s.conn.SetWriteDeadline(time.Now().Add(s.soket.Config.WritePeriod))
func WithWritePeriod(writePeriod time.Duration) ConfigParam {
	return func(c *Config) {
		c.WritePeriod = writePeriod
	}
}

// After opening a socket connection
// the socket needs to be kept alive
// so, after some time we should do pinging
func WithPingPeriod(pingPeriod time.Duration) ConfigParam {
	return func(c *Config) {
		c.PingPeriod = pingPeriod
	}
}

// After opening a socket connection
// the socket needs to be kept alive
// so, after some time we should do pinging
func WithPongPeriod(pongPeriod time.Duration) ConfigParam {
	return func(c *Config) {
		c.PongPeriod = pongPeriod
	}
}

// maxMessageSize sets the maximum size in bytes for a message read
// s.conn.SetReadLimit(int64(s.soket.Config.MaxMessageSize))
func WithMaxMessageSize(maxMessageSize int) ConfigParam {
	return func(c *Config) {
		c.MaxMessageSize = maxMessageSize
	}
}
