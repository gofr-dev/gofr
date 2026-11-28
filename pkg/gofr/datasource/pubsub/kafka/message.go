package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"

	"gofr.dev/pkg/gofr/datasource/pubsub"
)

type kafkaMessage struct {
	msg    *kafka.Message
	reader Reader
	logger pubsub.Logger
	// release drops the reader this message came from, so that the next Subscribe starts a new one.
	release func(Reader)
}

func newKafkaMessage(msg *kafka.Message, reader Reader, logger pubsub.Logger) *kafkaMessage {
	return &kafkaMessage{
		msg:    msg,
		reader: reader,
		logger: logger,
	}
}

func (kmsg *kafkaMessage) Commit() {
	if kmsg.reader != nil {
		err := kmsg.reader.CommitMessages(context.Background(), *kmsg.msg)
		if err != nil {
			kmsg.logger.Errorf("unable to commit message on kafka")
		}
	}
}

// Redeliver makes the consumer group deliver this message again. It commits the message's own offset as the
// group's position (CommitMessages stores Offset+1) and releases the reader, which has already fetched past the
// message. The next Subscribe starts a new reader that resumes from that position, even when the group had no
// committed offset yet and PUBSUB_OFFSET would otherwise start it at the latest message.
func (kmsg *kafkaMessage) Redeliver() {
	if kmsg.reader == nil {
		return
	}

	rewind := kafka.Message{Topic: kmsg.msg.Topic, Partition: kmsg.msg.Partition, Offset: kmsg.msg.Offset - 1}

	if err := kmsg.reader.CommitMessages(context.Background(), rewind); err != nil {
		kmsg.logger.Errorf("unable to reset the kafka offset to redeliver the message: %v", err)
	}

	if kmsg.release != nil {
		kmsg.release(kmsg.reader)
	}
}
