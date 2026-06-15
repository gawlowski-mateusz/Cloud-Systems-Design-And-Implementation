// Package snspub publishes notification emails through an Amazon SNS topic that
// has the recipient's address subscribed. SES is unavailable in the Learner Lab
// (it forbids verifying identities), so SNS email delivery is used instead.
package snspub

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type Publisher struct {
	client   *sns.Client
	topicARN string
}

func New(client *sns.Client, topicARN string) *Publisher {
	return &Publisher{client: client, topicARN: topicARN}
}

func (p *Publisher) Publish(ctx context.Context, subject, message string) error {
	_, err := p.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(p.topicARN),
		Subject:  aws.String(subject), // SNS subjects must be ASCII, <= 100 chars
		Message:  aws.String(message),
	})
	return err
}
