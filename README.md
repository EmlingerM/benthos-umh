# benthos-umh

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache2.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![GitHub Actions](https://github.com/united-manufacturing-hub/benthos-umh/workflows/main/badge.svg)](https://github.com/united-manufacturing-hub/benthos-umh/actions)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Funited-manufacturing-hub%2Fbenthos-umh.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Funited-manufacturing-hub%2Fbenthos-umh?ref=badge_shield)

Welcome to the benthos-umh repository! This is a version of benthos maintained by the United Manufacturing Hub (UMH) to provide seamless shopfloor integration with the [Unified Namespace](https://learn.umh.app/lesson/introduction-into-it-ot-unified-namespace/) (MQTT/Kafka). Our goal is to enhance the integration of IT and OT tools for engineers while avoiding vendor lock-in and streamlining data management processes.

## Description

`benthos-umh` is a Docker container designed to facilitate seamless shopfloor integration with the Unified Namespace (MQTT/Kafka). It is part of the United Manufacturing Hub project and offers the following features:

- Simple deployment in Docker, docker-compose, and Kubernetes
- Connects to an OPC-UA server, browses selected nodes, and forwards all sub-nodes in 1-second intervals.
- Can connect to an S7 server, and read pre-defined addresses from it
- Contains community supported plugins, such as SMTP and Beckhoff ADS
- Supports a wide range of outputs, from the Unified Namespace (MQTT and Kafka) to HTTP, AMQP, Redis, NATS, SQL, MongoDB, Cassandra, or AWS S3. Check out the official [benthos output library](https://benthos.dev/docs/components/outputs/about)
- Fully customizable messages using the benthos processor library: implement Report-by-Exception (RBE) / message deduplication, modify payloads and add timestamps using bloblang, apply protobuf (and therefore SparkplugB), and explore many more options
- Integrates with modern IT landscape, providing metrics, logging, tracing, versionable configuration, and more
- Entirely open-source (Apache 2.0) and free-to-use

We encourage you to try out `benthos-umh` and explore the broader [United Manufacturing Hub](https://www.umh.app) project for a comprehensive solution to your industrial data integration needs.

### Additional Plugins
If you are familiar with benthos, you know it is a powerful tool with many input, output, and processor plugins. We have added some additional plugins to benthos-umh, which are not part of the official benthos release. These plugins are:
- (UMH) s7comm: An input plugin to read data from Siemens S7 PLCs. See [further below](#s7comm) for more information.
- (UMH) opcua: An input plugin to read data from OPC UA servers. See [further below](#opc-ua)  for more information.
- (community) smtp: A output plugin to send emails via SMTP. See [DanielH's repo](https://github.com/RuneRoven/benthosSMTP) for more information.
- (community) ADS: An input plugin to read data from Beckhoff PLCs via ADS. See [further below](#beckhoff-ads) or at [DanielH's  repo](https://github.com/RuneRoven/benthosADS) for more information.

The plugins marked with "UMH" are developed and maintained by us. The ones marked as "community" are developed by the community and are not maintained by us.

## Usage

### Standalone

To use benthos-umh in standalone mode with Docker, follow the instructions below (using OPC UA as an example).

1. Create a new file called benthos.yaml with the provided content

    ```yaml
    ---
    input:
      opcua:
        endpoint: 'opc.tcp://localhost:46010'
        nodeIDs: ['ns=2;s=IoTSensors']

    pipeline:
      processors:
        - bloblang: |
            root = {
              meta("opcua_path"): this,
              "timestamp_ms": (timestamp_unix_nano() / 1000000).floor()
            }

    output:
      mqtt:
        urls:
          - 'localhost:1883'
        topic: 'ia/raw/opcuasimulator/${! meta("opcua_path") }'
        client_id: 'benthos-umh'
    ```

2. Execute the docker run command to start a new benthos-umh container
    `docker run --rm --network="host" -v '<absolute path to your file>/benthos.yaml:/benthos.yaml' ghcr.io/united-manufacturing-hub/benthos-umh:latest`

### With the United Manufacturing Hub (Kubernetes & Kafka)

To deploy benthos-umh with the United Manufacturing Hub and its OPC-UA simulator, use the provided Kubernetes manifests in UMHLens/OpenLens.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: benthos-1-config
  namespace: united-manufacturing-hub
  labels:
    app: benthos-1
data:
  benthos.yaml: |-
    input:
      umh_input_opcuasimulator: {}
    pipeline:
      processors:
        - bloblang: |
            root = {
              meta("opcua_path"): this,
              "timestamp_ms": (timestamp_unix_nano() / 1000000).floor()
            }
    output:
      umh_output:
        topic: 'ia.raw.${! meta("opcua_path") }'
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: benthos-1-deployment
  namespace: united-manufacturing-hub
  labels:
    app: benthos-1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: benthos-1
  template:
    metadata:
      labels:
        app: benthos-1
    spec:
      containers:
        - name: benthos-1
          image: "ghcr.io/united-manufacturing-hub/benthos-umh:latest"
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 4195
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /ping
              port: http
          readinessProbe:
            httpGet:
              path: /ready
              port: http
          volumeMounts:
            - name: config
              mountPath: "/benthos.yaml"
              subPath: "benthos.yaml"
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: benthos-1-config
```

### OPC UA

The plugin is designed to browse and subscribe to all child nodes within a folder for each configured NodeID, provided that the NodeID represents a folder. It features a recursion depth of up to 10 levels, enabling thorough exploration of nested folder structures. The browsing specifically targets nodes organized under the OPC UA 'Organizes' relationship type, intentionally excluding nodes under 'HasProperty' and 'HasComponent' relationships. Additionally, the plugin does not browse Objects represented by red, blue, or green cube icons in UAExpert.

Subscriptions are selectively managed, with tags having a DataType of null being excluded from subscription. Also, by default, the plugin does not subscribe to the properties of a tag, such as minimum and maximum values.

#### Datatypes
The plugin has been rigorously tested with an array of datatypes, both as single values and as arrays. The following datatypes have been verified for compatibility:

- `Boolean`
- `Byte`
- `DateTime`
- `Double`
- `Enumeration`
- `ExpandedNodeId`
- `Float`
- `Guid`
- `Int16`
- `Int32`
- `Int64`
- `Integer`
- `LocalizedText`
- `NodeId`
- `Number`
- `QualifiedName`
- `SByte`
- `StatusCode`
- `String`
- `UInt16`
- `UInt32`
- `UInt64`
- `UInteger`
- `ByteArray`
- `ByteString`
- `Duration`
- `LocaleId`
- `UtcTime`
- `Variant`
- `XmlElement`

There are specific datatypes which are currently not supported by the plugin and attempting to use them will result in errors. These include:

- Two-dimensional arrays
- UA Extension Objects
- Variant arrays (Arrays with multiple different datatypes)


#### Authentication and Security

In benthos-umh, we design security and authentication to be as robust as possible while maintaining flexibility. The software automates the process of selecting the highest level of security offered by an OPC-UA server for the selected Authentication Method, but the user can specify their own Security Policy / Security Mode if they want (see below at Configuration options)

##### Supported Authentication Methods

- **Anonymous**: No extra information is needed. The connection uses the highest security level available for anonymous connections.
- **Username and Password**: Specify the username and password in the configuration. The client opts for the highest security level that supports these credentials.
- **Certificate (Future Release)**: Certificate-based authentication is planned for future releases.

### Metadata outputs

The plugin provides metadata for each message, that can be used to create a topic for the output, as shown in the example above. The metadata can also be used to create a unique identifier for each message, which is useful for deduplication.

| Metadata                 | Description                                                                                                                                          |
|--------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| `opcua_tag_name`         | The sanitized ID of the Node that sent the message. This is always unique between nodes                                                              |
| `opcua_tag_group`        | A dot-separated path to the tag, created by joining the BrowseNames.                                                                                 |
| `opcua_tag_type`         | The data type of the node optimized for benthos, which can be either a number, string or bool. For the original one, check out `opcua_attr_datatype` |
| `opcua_source_timestamp` | The SourceTimestamp of the OPC UA node                                                                                                               |
| `opcua_server_timestamp` | The ServerTimestamp of the OPC UA node                                                                                                               |
| `opcua_attr_nodeid`      | The NodeID attribute of the Node as a string                                                                                                         |
| `opcua_attr_nodeclass`   | The NodeClass attribute of the Node as a string                                                                                                      |
| `opcua_attr_browsename`  | The BrowseName attribute of the Node as a string                                                                                                     |
| `opcua_attr_description` | The Description attribute of the Node as a string                                                                                                    |
| `opcua_attr_accesslevel` | The AccessLevel attribute of the Node as a string                                                                                                    |
| `opcua_attr_datatype`    | The DataType attribute of the Node as a string                                                                                                       |

Taking as example the following OPC-UA structure:

```text
Root
└── ns=2;s=FolderNode
    ├── ns=2;s=Tag1
    ├── ns=2;s=Tag2
    └── ns=2;s=SubFolder
        ├── ns=2;s=Tag3
        └── ns=2;s=Tag4
```

Subscribing to `ns=2;s=FolderNode` would result in the following metadata:

| `opcua_tag_name` | `opcua_tag_group`      |
|------------------|------------------------|
| `Tag1`           | `FolderNode`           |
| `Tag2`           | `FolderNode`           |
| `Tag3`           | `FolderNode.SubFolder` |
| `Tag4`           | `FolderNode.SubFolder` |

### Configuration Options

The following options can be specified in the `benthos.yaml` configuration file:

```yaml
input:
  opcua:
    endpoint: 'opc.tcp://localhost:46010'
    nodeIDs: ['ns=2;s=IoTSensors']
    username: 'your-username'  # optional (default: unset)
    password: 'your-password'  # optional (default: unset)
    insecure: false | true # DEPRECATED, see below
    securityMode: None | Sign | SignAndEncrypt # optional (default: unset)
    securityPolicy: None | Basic256Sha256  # optional (default: unset)
    subscribeEnabled: false | true # optional (default: false)
```

##### Endpoint

You can specify the endpoint in the configuration file. Node endpoints are automatically discovered and selected based on the authentication method.

```yaml
input:
  opcua:
    endpoint: 'opc.tcp://localhost:46010'
    nodeIDs: ['ns=2;s=IoTSensors']
```

##### Node IDs

You can specify the node IDs in the configuration file (currently only namespaced node IDs are supported):

```yaml
input:
  opcua:
    endpoint: 'opc.tcp://localhost:46010'
    nodeIDs: ['ns=2;s=IoTSensors']
```

##### Username and Password

If you want to use username and password authentication, you can specify them in the configuration file:

```yaml
input:
  opcua:
    endpoint: 'opc.tcp://localhost:46010'
    nodeIDs: ['ns=2;s=IoTSensors']
    username: 'your-username'
    password: 'your-password'
```

##### Security Mode and Security Policy

Security Mode: This defines the level of security applied to the messages. The options are:
- None: No security is applied; messages are neither signed nor encrypted.
- Sign: Messages are signed for integrity and authenticity but not encrypted.
- SignAndEncrypt: Provides the highest security level where messages are both signed and encrypted.

Security Policy: Specifies the set of cryptographic algorithms used for securing messages. This includes algorithms for encryption, decryption, and signing of messages. Currently only Basic256Sha256 is allowed.

While the security mode and policy are automatically selected based on the endpoint and authentication method, you have the option to override this by specifying them in the configuration file:

```yaml
input:
  opcua:
    endpoint: 'opc.tcp://localhost:46010'
    nodeIDs: ['ns=2;s=IoTSensors']
    securityMode: SignAndEncrypt
    securityPolicy: Basic256Sha256
```

##### Insecure Mode

This is now deprecated. By default, benthos-umh will now connect via SignAndEncrypt and Basic256Sha256 and if this fails it will fall back to insecure mode.

##### Pull and Subscribe Methods

Benthos-umh supports two modes of operation: pull and subscribe. In pull mode, it pulls all nodes every second, regardless of changes. In subscribe mode, it only sends data when there's a change in value, reducing unnecessary data transfer.

| Method    | Advantages                                                                                                                                                                                                                                    | Disadvantages                                                                                                        |
|-----------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------|
| Pull      | - Provides real-time data visibility, e.g., in MQTT Explorer. <br> - Clearly differentiates between 'no data received' and 'value did not change' scenarios, which can be crucial for documentation and proving the OPC-UA client's activity. | - Results in higher data throughput as it pulls all nodes every second, regardless of changes.                       |
| Subscribe | - Data is sent only when there's a change in value, reducing unnecessary data transfer.                                                                                                                                                       | - Less visibility into real-time data status, and it's harder to differentiate between no data and unchanged values. |

```yaml
input:
  opcua:
    endpoint: 'opc.tcp://localhost:46010'
    nodeIDs: ['ns=2;s=IoTSensors']
    subscribeEnabled: true
```

### S7comm

This input is tailored for the S7 communication protocol, facilitating a direct connection with S7-300, S7-400, S7-1200, and S7-1500 series PLCs.

For more modern PLCs like the S7-1200 and S7-1500 the following two changes need to be done to use them:
1. "Optimized block access" must be disabled for the DBs we want to access
2. In the "Protection" section of the CPU Properties, enable the "Permit access with PUT/GET" checkbox

#### Configuration
```yaml
input:
  s7comm:
    tcpDevice: '192.168.0.1' # IP address of the S7 PLC
    rack: 0                  # Rack number of the PLC. Defaults to 0
    slot: 1                  # Slot number of the PLC. Defaults to 1
    batchMaxSize: 480         # Maximum number of addresses per batch request. Defaults to 480
    timeout: 10             # Timeout in seconds for connections and requests. Default to 10
    disableCPUInfo: false # Set this to true to not fetch CPU information from the PLC. Should be used when you get the error 'Failed to get CPU information'
    addresses:               # List of addresses to read from
      - "DB1.DW20"     # Accesses a double word at location 20 in data block 1
      - "DB1.S30.10"   # Accesses a 10-byte string at location 30 in data block 1
```

#### Configuration Parameters

- **tcpDevice**: IP address of the Siemens S7 PLC.
- **rack**: Identifies the physical location of the CPU within the PLC rack.
- **slot**: Identifies the specific CPU slot within the rack.
- **batchMaxSize**: Maximum count of addresses bundled in a single batch request. This affects the PDU size.
- **timeout**: Timeout duration in milliseconds for connection attempts and read requests.
- **disableCPUInfo**: Set this to true to not fetch CPU information from the PLC. Should be used when you get the error 'Failed to get CPU information'
- **addresses**: Specifies the list of addresses to read. The format for addresses is `<area>.<type><address>[.extra]`, where:
  - `area`: Specifies the direct area access, e.g., "DB1" for data block one. Supported areas include inputs (`PE`), outputs (`PA`), Merkers (`MK`), DB (`DB`), counters (`C`), and timers (`T`).
  - `type`: Indicates the data type, such as bit (`X`), byte (`B`), word (`W`), double word (`DW`), integer (`I`), double integer (`DI`), real (`R`), date-time (`DT`), and string (`S`). Some types require an 'extra' parameter, e.g., the bit number for `X` or the maximum length for `S`.

#### Output

Similar to the OPC UA input, this outputs for each address a single message with the payload being the value that was read. To distinguish messages, you can use meta("s7_address") in a following benthos bloblang processor.

### Beckhoff ADS
Input for Beckhoff's ADS protocol. Supports batch reading and notifications. Beckhoff recommends limiting notifications to approximately 500 to avoid overloading the controller.
This input only supports symbols and not direct addresses.

**This plugin is community supported only. If you encounter any issues, check out the [original repository](https://github.com/RuneRoven/benthosADS) for more information, or ask around in our Discord.**

```yaml
---
input:
  ads:
    targetIP: '192.168.3.70'        # IP address of the PLC
    targetAMS: '5.3.69.134.1.1'     # AMS net ID of the target
    targetPort: 48898               # Port of the target internal gateway
    runtimePort: 801                # Runtime port of PLC system
    hostAMS: '192.168.56.1.1.1'     # Host AMS net ID. Usually the IP address + .1.1
    hostPort: 10500                 # Host port
    readType: 'interval'            # Read type, interval or notification
    maxDelay: 100                   # Max delay for sending notifications in ms
    cycleTime: 100                  # Cycle time for notification handler in ms
    intervalTime: 1000              # Interval time for reading in ms
    upperCase: true                 # Convert symbol names to all uppercase for older PLCs
    logLevel: "disabled"            # Log level for ADS connection
    symbols:                        # List of symbols to read from
      - "MAIN.MYBOOL"               # variable in the main program
      - "MAIN.MYTRIGGER:0:10"       # variable in the main program with 0ms max delay and 10ms cycleTime
      - "MAIN.SPEEDOS"
      - ".superDuperInt"            # Global variable
      - ".someStrangeVar"

pipeline:
  processors:
    - bloblang: |
        root = {
          meta("symbol_name"): this,
          "timestamp_ms": (timestamp_unix_nano() / 1000000).floor()
        }
output:
  stdout: {}

logger:
  level: ERROR
  format: logfmt
  add_timestamp: true
  ```

#### Connection to ADS
Connecting to an ADS device involves routing traffic through a router using the AMS net ID.
There are basically 2 ways for setting up the connection. One approach involves using the Twincat connection manager to locally scan for the device on the host and add a connection using the correct PLC credentials. The other way is to log in to the PLC using the Twincat system manager and add a static route from the PLC to the client. This is the preferred way when using benthos on a Kubernetes cluster since you have no good way of installing the connection manager.

#### Configuration Parameters
- **targetIP**: IP address of the PLC
- **targetAMS**: AMS net ID of the target
- **targetPort**: Port of the target internal gateway
- **runtimePort**: Runtime port of PLC system,  800 to 899. Twincat 2 uses ports 800 to 850, while Twincat 3 is recommended to use ports 851 to 899. Twincat 2 usually have 801 as default and Twincat 3 uses 851
- **hostAMS**: Host AMS net ID. Usually the IP address + .1.1
- **hostPort**: Host port
- **readType**: Read type for the symbols. Interval means benthos reads all symbols at a specified interval and notification is a function in the PLC where benthos sends a notification request to the PLC and the PLC adds the symbol to its internal notification system and sends data whenever there is a change.
- **maxDelay**: Default max delay for sending notifications in ms. Sets a maximum time for how long after the change the PLC must send the notification
- **cycleTime**: Default cycle time for notification handler in ms. Tells the notification handler how often to scan for changes. For symbols like triggers that is only true or false for 1 PLC cycle it can be necessary to use a low value.
- **intervalTime**: Interval time for reading in ms. For reading batches of symbols this sets the time between readings
- **upperCase**: Converts symbol names to all uppercase for older PLCs. For Twincat 2 this is often necessary.
- **logLevel**: Log level for ADS connection sets the log level of the internal log function for the underlying ADS library
- **symbols**: List of symbols to read from in the format <function.variable:maxDelay:cycleTime>, e.g., "MAIN.MYTRIGGER:0:10" is a variable in the main program with 0ms max delay and 10ms cycle time,  "MAIN.MYBOOL" is a variable in the main program with no extra arguments, so it will use the default max delay and cycle time. ".superDuperInt" is a global variable with no extra arguments. All global variables must start with a <.> e.g., ".someStrangeVar"

#### Output

Similar to the OPC UA input, this outputs for each address a single message with the payload being the value that was read. To distinguish messages, you can use meta("symbol_name") in a following benthos bloblang processor.

## Testing

We execute automated tests and verify that benthos-umh works against various targets. All tests are started with `make test`, but might require environment parameters in order to not be skipped.

Some of these tests are executed with a local GitHub runner called "hercules", which is connected to an isolated testing network.

### Target: WAGO PFC100 (OPC UA)

Model number: 750-8101

Requires:
- TEST_WAGO_ENDPOINT_URI
- TEST_WAGO_USERNAME
- TEST_WAGO_PASSWORD

### Target: Microsoft OPC UA Simulator (OPC UA)

Docker tag: mcr.microsoft.com/iotedge/opc-plc:2.9.11

Requires:
- TEST_OPCUA_SIMULATOR

### Target: Prosys OPC UA Simulator (OPC UA)

Version: 5.4.6-148

Requires:
- TEST_PROSYS_ENDPOINT_URI

This requires additional to have the simulator setup somewhere (e.g., locally on your PC) and pointing the test towards it. This is not included in any CI andm ust be run manually.

### Target: Siemens S7-1200 (OPC UA)

Model number: SIMATIC S7-1200 6ES7211-1AE40-0XB0

Requires:
- TEST_S7_ENDPOINT_URI

### Target: Unit Tests (OPC UA)

Requires:
- TEST_OPCUA_UNITTEST

### Target: Siemens S7-1200 (S7comm)

Model number: SIMATIC S7-1200 6ES7211-1AE40-0XB0

Requires:
- TEST_S7_TCPDEVICE
- TEST_S7_RACK
- TEST_S7_SLOT

### Target: Unit Tests (S7comm)

Requires:
- TEST_S7COMM_UNITTEST

## Development

### Quickstart

Follow the steps below to set up your development environment and run tests:

```
git clone https://github.com/united-manufacturing-hub/benthos-umh.git
cd serverless-stack
nvm install
npm install
sudo apt-get install zip
echo 'deb [trusted=yes] https://repo.goreleaser.com/apt/ /' | sudo tee /etc/apt/sources.list.d/goreleaser.list
sudo apt update
sudo apt install goreleaser
make
npm test
```

### Additional Checks and Commands

#### Gitpod and Tailscale

By default, when opening the repo in Gitpod, everything that you need should start automatically. If you want to connect to our local PLCs in our office, you can use tailscale, which you will be prompted to install.
See also: <https://www.gitpod.io/docs/integrations/tailscale>

#### For Go Code

1. **Linting**: Run `make lint` to check for linting errors. If any are found, you can automatically fix them by running `make format`.

2. **Unit Tests**: Run `make test` to execute all Go unit tests.

#### For Other Code Types (Including Config Files)

1. **Benthos Tests**: Use `npm run test` to run all Benthos tests for configuration files. Note: We currently do not have these tests. [Learn more](https://www.benthos.dev/docs/configuration/unit_testing/).

2. **Linting**: Run `npm run lint` to check all files, including YAML files, for linting errors. To automatically fix these errors, run `npm run format`.

## License

All source code is distributed under the APACHE LICENSE, VERSION 2.0. See LICENSE for more information.


[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Funited-manufacturing-hub%2Fbenthos-umh.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Funited-manufacturing-hub%2Fbenthos-umh?ref=badge_large)

## Contact

Feel free to provide us feedback on our [Discord channel](https://discord.gg/F9mqkZnm9d).

For more information about the United Manufacturing Hub, visit [UMH Systems GmbH](https://www.umh.app). If you haven't worked with the United Manufacturing Hub before, [give it a try](https://umh.docs.umh.app/docs/getstarted/installation/)! Setting it up takes only a matter of minutes.
