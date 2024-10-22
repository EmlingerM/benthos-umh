# Copyright 2023 UMH Systems GmbH
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

all: clean target

clean:
	@rm -rf target tmp/bin tmp/benthos-*.zip

target:
	@mkdir -p tmp/bin
	@goreleaser build --single-target --snapshot --id benthos \
		--output ./tmp/bin/benthos

test:
	@ginkgo -r --output-interceptor-mode=none --github-output -v -trace -p --randomize-all --cover --label-filter="now" ./opcua_plugin/...

lint:
	@golangci-lint run

format:
	@golangci-lint run --fix

update-benthos:
	@go get github.com/redpanda-data/connect/public/bundle/free/v4@latest && \
  go get github.com/redpanda-data/connect/v4@latest && \
  go get github.com/redpanda-data/benthos/v4@latest && \
  go mod tidy

.PHONY: clean target test lint format update-benthos
