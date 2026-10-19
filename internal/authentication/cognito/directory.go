// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cognito

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

type DirectoryAPI interface {
	AdminCreateUser(context.Context, *cognitoidentityprovider.AdminCreateUserInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminCreateUserOutput, error)
	AdminGetUser(context.Context, *cognitoidentityprovider.AdminGetUserInput, ...func(*cognitoidentityprovider.Options)) (*cognitoidentityprovider.AdminGetUserOutput, error)
}
type Directory struct {
	PoolID string
	Client DirectoryAPI
}

var errDirectory = errors.New("directory operation could not be confirmed")

func directorySubject(attrs []types.AttributeType) string {
	subject := ""
	for _, a := range attrs {
		if aws.ToString(a.Name) == "sub" {
			if subject != "" {
				return ""
			}
			subject = aws.ToString(a.Value)
		}
	}
	return subject
}
func (d Directory) Create(ctx context.Context, email string) (string, error) {
	if d.PoolID == "" || d.Client == nil {
		return "", errDirectory
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// Do not steal an existing alias, set passwords, or mark email verified.
	result, err := d.Client.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{UserPoolId: aws.String(d.PoolID), Username: aws.String(email), MessageAction: types.MessageActionTypeSuppress, ForceAliasCreation: false, UserAttributes: []types.AttributeType{{Name: aws.String("email"), Value: aws.String(email)}}})
	if err != nil || result == nil || result.User == nil || !result.User.Enabled || result.User.UserStatus != types.UserStatusTypeForceChangePassword {
		return "", errDirectory
	}
	subject := directorySubject(result.User.Attributes)
	if subject == "" {
		return "", errDirectory
	}
	return subject, nil
}
func (d Directory) Send(ctx context.Context, subject string) error {
	if d.PoolID == "" || d.Client == nil || subject == "" {
		return errDirectory
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	user, err := d.Client.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{UserPoolId: aws.String(d.PoolID), Username: aws.String(subject)})
	if err != nil || user == nil || !user.Enabled || user.UserStatus != types.UserStatusTypeForceChangePassword || directorySubject(user.UserAttributes) != subject || aws.ToString(user.Username) == "" {
		return errDirectory
	}
	result, err := d.Client.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{UserPoolId: aws.String(d.PoolID), Username: user.Username, MessageAction: types.MessageActionTypeResend, DesiredDeliveryMediums: []types.DeliveryMediumType{types.DeliveryMediumTypeEmail}, ForceAliasCreation: false})
	if err != nil || result == nil || result.User == nil || directorySubject(result.User.Attributes) != subject {
		return errDirectory
	}
	return nil
}
