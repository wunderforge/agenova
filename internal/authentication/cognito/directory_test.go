// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cognito

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

type directoryFixture struct {
	t         *testing.T
	calls     int
	confirmed bool
}

func (f *directoryFixture) AdminCreateUser(_ context.Context, in *sdk.AdminCreateUserInput, _ ...func(*sdk.Options)) (*sdk.AdminCreateUserOutput, error) {
	f.calls++
	if in.ForceAliasCreation || in.TemporaryPassword != nil || aws.ToString(in.UserPoolId) != "pool" {
		f.t.Fatal("unsafe directory invitation")
	}
	if f.calls == 1 {
		if in.MessageAction != types.MessageActionTypeSuppress {
			f.t.Fatal("email sent before durable binding")
		}
	} else {
		if in.MessageAction != types.MessageActionTypeResend || len(in.DesiredDeliveryMediums) != 1 || in.DesiredDeliveryMediums[0] != types.DeliveryMediumTypeEmail || aws.ToString(in.Username) != "internal-name" {
			f.t.Fatal("wrong delivery binding")
		}
	}
	return &sdk.AdminCreateUserOutput{User: &types.UserType{Enabled: true, UserStatus: types.UserStatusTypeForceChangePassword, Attributes: []types.AttributeType{{Name: aws.String("sub"), Value: aws.String("subject")}}}}, nil
}
func (f *directoryFixture) AdminGetUser(_ context.Context, in *sdk.AdminGetUserInput, _ ...func(*sdk.Options)) (*sdk.AdminGetUserOutput, error) {
	if aws.ToString(in.Username) != "subject" {
		f.t.Fatal("lookup lost subject binding")
	}
	status := types.UserStatusTypeForceChangePassword
	if f.confirmed {
		status = types.UserStatusTypeConfirmed
	}
	return &sdk.AdminGetUserOutput{Enabled: true, Username: aws.String("internal-name"), UserStatus: status, UserAttributes: []types.AttributeType{{Name: aws.String("sub"), Value: aws.String("subject")}}}, nil
}
func TestDirectoryBindsSubjectAndDoesNotResetAcceptedUser(t *testing.T) {
	client := &directoryFixture{t: t}
	d := Directory{PoolID: "pool", Client: client}
	subject, err := d.Create(context.Background(), "person@example.com")
	if err != nil || subject != "subject" {
		t.Fatal(err)
	}
	if err = d.Send(context.Background(), subject); err != nil {
		t.Fatal(err)
	}
	client.confirmed = true
	if err = d.Send(context.Background(), subject); err == nil || client.calls != 2 {
		t.Fatal("confirmed account password was reset")
	}
}
