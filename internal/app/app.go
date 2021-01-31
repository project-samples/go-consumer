package app

import (
	"context"
	"reflect"

	"github.com/common-go/health"
	"github.com/common-go/kafka"
	"github.com/common-go/mongo"
	"github.com/common-go/mq"
	v "github.com/common-go/validator"
	"github.com/common-go/zap"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap/zapcore"
)

type ApplicationContext struct {
	Consumer        mq.Consumer
	ConsumerHandler mq.ConsumerHandler
	HealthHandler   *health.HealthHandler
}

func NewApp(ctx context.Context, root Root) (*ApplicationContext, error) {
	logger, er0 := log.Initialize(root.Log)
	if er0 != nil {
		return nil, er0
	}
	mongoDb, er1 := mongo.SetupMongo(ctx, root.Mongo)
	if er1 != nil {
		log.Error(ctx, "Cannot connect to MongoDB: Error: "+er1.Error())
		return nil, er1
	}

	logError := log.ErrorMsg
	var logInfo func(context.Context, string)
	if logger.Core().Enabled(zapcore.InfoLevel) {
		logInfo = log.InfoMsg
	}

	consumer, er2 := kafka.NewConsumerByConfig(root.Consumer.KafkaConsumer, true)
	if er2 != nil {
		log.Error(ctx, "Cannot create a new consumer: Error: "+er2.Error())
		return nil, er2
	}
	userTypeOf := reflect.TypeOf(User{})
	writer := mongo.NewMongoInserter(mongoDb, "users")
	validator := mq.NewValidator(userTypeOf, NewUserValidator())

	mongoChecker := mongo.NewHealthChecker(mongoDb)
	consumerChecker := kafka.NewKafkaHealthChecker(root.Consumer.KafkaConsumer.Brokers)
	var checkers []health.HealthChecker
	var consumerCaller mq.ConsumerHandler
	if root.KafkaProducer != nil {
		producer, er3 := kafka.NewProducerByConfig(*root.KafkaProducer, true)
		if er3 != nil {
			log.Error(ctx, "Cannot new a new producer. Error:"+er3.Error())
			return nil, er3
		}
		retryService := mq.NewMqRetryService(producer, logError, logInfo)
		consumerCaller = mq.NewConsumerHandlerByConfig(root.Consumer.Config, userTypeOf, writer, retryService, validator, nil, logError, logInfo)
		producerChecker := kafka.NewKafkaHealthChecker(root.KafkaProducer.Brokers)
		checkers = []health.HealthChecker{mongoChecker, consumerChecker, producerChecker}
	} else {
		checkers = []health.HealthChecker{mongoChecker, consumerChecker}
		consumerCaller = mq.NewConsumerHandlerWithRetryConfig(userTypeOf, writer, validator, root.Retry, true, logError, logInfo)
	}

	handler := health.NewHealthHandler(checkers)
	return &ApplicationContext{
		Consumer:        consumer,
		ConsumerHandler: consumerCaller,
		HealthHandler:   handler,
	}, nil
}

func NewUserValidator() v.Validator {
	validator := v.NewDefaultValidator()
	validator.CustomValidateList = append(validator.CustomValidateList, v.CustomValidate{Fn: CheckActive, Tag: "active"})
	return validator
}

func CheckActive(fl validator.FieldLevel) bool {
	return fl.Field().Bool()
}
