/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/internal/balancer/stub"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/serviceconfig"
	"google.golang.org/grpc/status"
)

const happyBalancerName = "happy balancer"

func init() {
	// Register a balancer that never returns an error from
	// UpdateClientConnState, and doesn't do anything else either.
	bf := stub.BalancerFuncs{
		UpdateClientConnState: func(*stub.BalancerData, balancer.ClientConnState) error {
			return nil
		},
	}
	stub.Register(happyBalancerName, bf)
}

// TestResolverErrorInBuild makes the resolver.Builder call into the ClientConn
// during the Build call. We use two separate mutexes in the code which make
// sure there is no data race in this code path, and also that there is no
// deadlock.
func (s) TestResolverErrorInBuild(t *testing.T) {
	r := manual.NewBuilderWithScheme("whatever")
	r.InitialState(resolver.State{ServiceConfig: &serviceconfig.ParseResult{Err: errors.New("resolver build err")}})

	cc, err := Dial(r.Scheme()+":///test.server", WithTransportCredentials(insecure.NewCredentials()), WithResolvers(r))
	if err != nil {
		t.Fatalf("Dial(_, _) = _, %v; want _, nil", err)
	}
	defer cc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var dummy int
	const wantMsg = "error parsing service config"
	const wantCode = codes.Unavailable
	if err := cc.Invoke(ctx, "/foo/bar", &dummy, &dummy); status.Code(err) != wantCode || !strings.Contains(status.Convert(err).Message(), wantMsg) {
		t.Fatalf("cc.Invoke(_, _, _, _) = %v; want status.Code()==%v, status.Message() contains %q", err, wantCode, wantMsg)
	}
}

func (s) TestServiceConfigErrorRPC(t *testing.T) {
	r := manual.NewBuilderWithScheme("whatever")

	cc, err := Dial(r.Scheme()+":///test.server", WithTransportCredentials(insecure.NewCredentials()), WithResolvers(r))
	if err != nil {
		t.Fatalf("Dial(_, _) = _, %v; want _, nil", err)
	}
	defer cc.Close()
	badsc := r.CC.ParseServiceConfig("bad config")
	r.UpdateState(resolver.State{ServiceConfig: badsc})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var dummy int
	const wantMsg = "error parsing service config"
	const wantCode = codes.Unavailable
	if err := cc.Invoke(ctx, "/foo/bar", &dummy, &dummy); status.Code(err) != wantCode || !strings.Contains(status.Convert(err).Message(), wantMsg) {
		t.Fatalf("cc.Invoke(_, _, _, _) = %v; want status.Code()==%v, status.Message() contains %q", err, wantCode, wantMsg)
	}
}
