package store

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoClient struct {
	client     *dynamodb.Client
	albumTable string
	photoTable string
}

func NewDynamoClient(ctx context.Context) (*DynamoClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &DynamoClient{
		client:     dynamodb.NewFromConfig(cfg),
		albumTable: "Albums",
		photoTable: "Photos",
	}, nil
}

// ---- Albums ----

func (d *DynamoClient) PutAlbum(ctx context.Context, albumID, title, description, owner string) error {
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.albumTable),
		Item: map[string]types.AttributeValue{
			"album_id":    &types.AttributeValueMemberS{Value: albumID},
			"title":       &types.AttributeValueMemberS{Value: title},
			"description": &types.AttributeValueMemberS{Value: description},
			"owner":       &types.AttributeValueMemberS{Value: owner},
		},
	})
	return err
}

func (d *DynamoClient) GetAlbum(ctx context.Context, albumID string) (map[string]string, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.albumTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	return extractStrings(out.Item, "album_id", "title", "description", "owner"), nil
}

func (d *DynamoClient) ListAlbums(ctx context.Context) ([]map[string]string, error) {
	var albums []map[string]string
	var lastKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName: aws.String(d.albumTable),
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}
		out, err := d.client.Scan(ctx, input)
		if err != nil {
			return nil, err
		}
		for _, item := range out.Items {
			albums = append(albums, extractStrings(item, "album_id", "title", "description", "owner"))
		}
		if out.LastEvaluatedKey == nil {
			break
		}
		lastKey = out.LastEvaluatedKey
	}
	return albums, nil
}

// ---- Photos ----

func (d *DynamoClient) PutPhoto(ctx context.Context, photoID, albumID string, seq int, status, url string) error {
	item := map[string]types.AttributeValue{
		"photo_id": &types.AttributeValueMemberS{Value: photoID},
		"album_id": &types.AttributeValueMemberS{Value: albumID},
		"seq":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", seq)},
		"status":   &types.AttributeValueMemberS{Value: status},
	}
	if url != "" {
		item["url"] = &types.AttributeValueMemberS{Value: url}
	}
	_, err := d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.photoTable),
		Item:      item,
	})
	return err
}

func (d *DynamoClient) GetPhoto(ctx context.Context, photoID string) (map[string]string, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.photoTable),
		Key: map[string]types.AttributeValue{
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	return extractStrings(out.Item, "photo_id", "album_id", "seq", "status", "url"), nil
}

func (d *DynamoClient) UpdatePhotoStatus(ctx context.Context, photoID, status, url string) error {
	expr := "SET #s = :s, #u = :u"
	exprNames := map[string]string{"#s": "status", "#u": "url"}
	exprVals := map[string]types.AttributeValue{
		":s": &types.AttributeValueMemberS{Value: status},
		":u": &types.AttributeValueMemberS{Value: url},
	}
	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(d.photoTable),
		Key:                       map[string]types.AttributeValue{"photo_id": &types.AttributeValueMemberS{Value: photoID}},
		UpdateExpression:          aws.String(expr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprVals,
	})
	return err
}

func (d *DynamoClient) DeletePhoto(ctx context.Context, photoID string) error {
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(d.photoTable),
		Key: map[string]types.AttributeValue{
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
	})
	return err
}

// ---- Helpers ----

func extractStrings(item map[string]types.AttributeValue, keys ...string) map[string]string {
	result := make(map[string]string)
	for _, k := range keys {
		if v, ok := item[k]; ok {
			switch val := v.(type) {
			case *types.AttributeValueMemberS:
				result[k] = val.Value
			case *types.AttributeValueMemberN:
				result[k] = val.Value
			}
		}
	}
	return result
}

func (d *DynamoClient) UpdatePhotoStatusIfExists(ctx context.Context, photoID, status, url string) error {
	_, _ = d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(d.photoTable),
		Key: map[string]types.AttributeValue{
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		UpdateExpression:         aws.String("SET #s = :s, #u = :u"),
		ConditionExpression:      aws.String("attribute_exists(photo_id)"),
		ExpressionAttributeNames: map[string]string{"#s": "status", "#u": "url"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":s": &types.AttributeValueMemberS{Value: status},
			":u": &types.AttributeValueMemberS{Value: url},
		},
	})
	// ConditionalCheckFailedException means photo was deleted — that's fine
	return nil
}