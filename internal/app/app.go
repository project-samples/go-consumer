package app

import (
	"context"
	"reflect"

	"github.com/common-go/health"
	"github.com/common-go/kafka"
	"github.com/common-go/log"
	"github.com/common-go/mongo"
	"github.com/common-go/mq"
	v "github.com/common-go/validator"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
)

type ApplicationContext struct {
	Consumer        mq.Consumer
	ConsumerHandler mq.ConsumerHandler
	HealthHandler   *health.HealthHandler
}

func NewApp(ctx context.Context, root Root) (*ApplicationContext, error) {
	log.Initialize(root.Log)
	mongoDb, er1 := mongo.SetupMongo(ctx, root.Mongo)
	if er1 != nil {
		log.Error(ctx, "Cannot connect to MongoDB: Error: "+er1.Error())
		return nil, er1
	}

	logError := log.ErrorMsg
	var logInfo func(context.Context, string)
	if logrus.IsLevelEnabled(logrus.InfoLevel) {
		logInfo = log.InfoMsg
	}

	consumer, er2 := kafka.NewConsumerByConfig(root.Consumer.KafkaConsumer, true)
	if er2 != nil {
		log.Error(ctx, "Cannot create a new consumer: Error: "+er2.Error())
		return nil, er2
	}
	userType := reflect.TypeOf(User{})
	mapper := mongo.NewMapper(userType)
	writer := mongo.NewInserter(mongoDb, "users", mapper.ModelToDb)
	validator := mq.NewValidator(userType, NewUserValidator().Validate)

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
		retryService := mq.NewMqRetryService(producer.Produce, logError, logInfo)
		consumerCaller = mq.NewConsumerHandlerByConfig(root.Consumer.Config, userType, writer.Write, retryService.Retry, validator.Validate, nil, logError, logInfo)
		producerChecker := kafka.NewKafkaHealthChecker(root.KafkaProducer.Brokers)
		checkers = []health.HealthChecker{mongoChecker, consumerChecker, producerChecker}
	} else {
		checkers = []health.HealthChecker{mongoChecker, consumerChecker}
		consumerCaller = mq.NewConsumerHandlerWithRetryConfig(userType, writer.Write, validator.Validate, root.Retry, true, logError, logInfo)
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
