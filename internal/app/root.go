package app

import (
	"github.com/common-go/health"
	"github.com/common-go/kafka"
	"github.com/common-go/log"
	"github.com/common-go/mongo"
	"github.com/common-go/mq"
)

type Root struct {
	Server        health.ServerConfig   `mapstructure:"server"`
	Log           log.Config            `mapstructure:"log"`
	Mongo         mongo.MongoConfig     `mapstructure:"mongo"`
	Retry         *mq.RetryConfig       `mapstructure:"retry"`
	Consumer      ConsumerConfig        `mapstructure:"consumer"`
	KafkaProducer *kafka.ProducerConfig `mapstructure:"kafka_producer"`
}

type ConsumerConfig struct {
	KafkaConsumer kafka.ConsumerConfig `mapstructure:"kafka"`
	Config        mq.ConsumerConfig    `mapstructure:"retry"`
}
