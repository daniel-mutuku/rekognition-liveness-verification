package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

type Dynamo struct {
	client *dynamodb.Client
	table  string
}

var _ verify.Store = (*Dynamo)(nil)

func NewDynamo(cfg aws.Config, table string) *Dynamo {
	return &Dynamo{client: dynamodb.NewFromConfig(cfg), table: table}
}

type item struct {
	ID                 string    `dynamodbav:"id"`
	Status             string    `dynamodbav:"status"`
	CreatedAt          time.Time `dynamodbav:"createdAt"`
	UpdatedAt          time.Time `dynamodbav:"updatedAt"`
	ExpiresAt          int64     `dynamodbav:"expiresAt"` // TTL needs epoch seconds
	LivenessSessionID  string    `dynamodbav:"livenessSessionId,omitempty"`
	LivenessAttempts   int       `dynamodbav:"livenessAttempts"`
	PhotoETag          string    `dynamodbav:"photoETag,omitempty"`
	ReferenceKey       string    `dynamodbav:"referenceKey,omitempty"`
	LivenessConfidence *float64  `dynamodbav:"livenessConfidence,omitempty"`
	Similarity         *float64  `dynamodbav:"similarity,omitempty"`
	Reasons            []string  `dynamodbav:"reasons,omitempty"`
	Version            int       `dynamodbav:"version"`
}

func toItem(v verify.Verification) item {
	it := item{
		ID: v.ID, Status: string(v.Status),
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, ExpiresAt: v.ExpiresAt.Unix(),
		LivenessSessionID: v.LivenessSessionID, LivenessAttempts: v.LivenessAttempts,
		PhotoETag: v.PhotoETag, ReferenceKey: v.ReferenceKey,
		Version: v.Version,
	}
	if v.Scores != nil {
		it.LivenessConfidence, it.Similarity = &v.Scores.Liveness, &v.Scores.Similarity
	}
	for _, r := range v.Reasons {
		it.Reasons = append(it.Reasons, string(r))
	}
	return it
}

func fromItem(it item) verify.Verification {
	v := verify.Verification{
		ID: it.ID, Status: verify.Status(it.Status),
		CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt, ExpiresAt: time.Unix(it.ExpiresAt, 0).UTC(),
		LivenessSessionID: it.LivenessSessionID, LivenessAttempts: it.LivenessAttempts,
		PhotoETag: it.PhotoETag, ReferenceKey: it.ReferenceKey,
		Version: it.Version,
	}
	if it.LivenessConfidence != nil && it.Similarity != nil {
		v.Scores = &verify.Scores{Liveness: *it.LivenessConfidence, Similarity: *it.Similarity}
	}
	for _, r := range it.Reasons {
		v.Reasons = append(v.Reasons, verify.Reason(r))
	}
	return v
}

func (d *Dynamo) Create(ctx context.Context, v *verify.Verification) error {
	c := *v
	c.Version = 1
	if err := d.put(ctx, c, "attribute_not_exists(#id)", map[string]string{"#id": "id"}, nil); err != nil {
		var failed *types.ConditionalCheckFailedException
		if errors.As(err, &failed) {
			return verify.ErrExists
		}
		return err
	}
	v.Version = 1
	return nil
}

func (d *Dynamo) Get(ctx context.Context, id string) (verify.Verification, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &d.table,
		Key:       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
		// Each step reads what the previous one just wrote.
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return verify.Verification{}, fmt.Errorf("dynamodb get %s: %w", id, err)
	}
	if out.Item == nil {
		return verify.Verification{}, verify.ErrNotFound
	}
	var it item
	if err := attributevalue.UnmarshalMap(out.Item, &it); err != nil {
		return verify.Verification{}, fmt.Errorf("decode %s: %w", id, err)
	}
	return fromItem(it), nil
}

func (d *Dynamo) Update(ctx context.Context, v *verify.Verification) error {
	c := *v
	c.Version++
	err := d.put(ctx, c, "#v = :v",
		map[string]string{"#v": "version"},
		map[string]types.AttributeValue{":v": &types.AttributeValueMemberN{Value: strconv.Itoa(v.Version)}},
	)
	var failed *types.ConditionalCheckFailedException
	if errors.As(err, &failed) {
		// ALL_OLD tells a missing item apart from a lost race.
		if len(failed.Item) == 0 {
			return verify.ErrNotFound
		}
		return verify.ErrConflict
	}
	if err != nil {
		return err
	}
	v.Version = c.Version
	return nil
}

func (d *Dynamo) put(ctx context.Context, v verify.Verification, cond string,
	names map[string]string, values map[string]types.AttributeValue) error {
	av, err := attributevalue.MarshalMap(toItem(v))
	if err != nil {
		return fmt.Errorf("encode %s: %w", v.ID, err)
	}
	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:                           &d.table,
		Item:                                av,
		ConditionExpression:                 &cond,
		ExpressionAttributeNames:            names,
		ExpressionAttributeValues:           values,
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	})
	if err != nil {
		return fmt.Errorf("dynamodb put %s: %w", v.ID, err)
	}
	return nil
}
