package opcua_plugin_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gopcua/opcua/ua"
	"os"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/united-manufacturing-hub/benthos-umh/opcua_plugin"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	_ "github.com/redpanda-data/benthos/v4/public/components/io"
	_ "github.com/redpanda-data/benthos/v4/public/components/pure"
	"github.com/redpanda-data/benthos/v4/public/service"
)

var _ = Describe("Debugging test", func() {

	var endpoint string

	BeforeEach(func() {
		endpoint = os.Getenv("TEST_DEBUG_ENDPOINT_URI")

		// Check if environment variables are set
		if endpoint == "" {
			Skip("Skipping test: environment variables not set")
			return
		}

	})

	When("using a yaml and stream builder", func() {

		It("should subscribe to all nodes and receive data", func() {

			// Create a new stream builder
			builder := service.NewStreamBuilder()

			// Create a new stream
			err := builder.AddInputYAML(fmt.Sprintf(`
opcua:
  endpoint: "%s"
  username: ""
  password: ""
  subscribeEnabled: true
  useHeartbeat: true
  nodeIDs:
    - i=84
`, endpoint))

			Expect(err).NotTo(HaveOccurred())

			err = builder.SetLoggerYAML(`level: off`)
			Expect(err).NotTo(HaveOccurred())

			err = builder.SetTracerYAML(`type: none`)
			Expect(err).NotTo(HaveOccurred())

			// Add a total message count consumer
			var count int64
			err = builder.AddConsumerFunc(func(c context.Context, m *service.Message) error {
				atomic.AddInt64(&count, 1)
				return err
			})

			stream, err := builder.Build()
			Expect(err).NotTo(HaveOccurred())

			timeout := time.Second * 45

			// Run the stream
			ctx, cncl := context.WithTimeout(context.Background(), timeout)
			go func() {
				_ = stream.Run(ctx)
			}()

			// Check if we received any messages continuously
			Eventually(
				func() int64 {
					return atomic.LoadInt64(&count)
				}, timeout).Should(BeNumerically(">", int64(0)))

			cncl()

		})
	})
})

var _ = Describe("Test Against Prosys Simulator", func() {

	var endpoint string

	BeforeEach(func() {
		endpoint = os.Getenv("TEST_PROSYS_ENDPOINT_URI")

		// Check if environment variables are set
		if endpoint == "" {
			Skip("Skipping test: environment variables not set")
			return
		}

	})

	Describe("OPC UA Server Information", func() {

		It("should connect to the server and retrieve server information", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			var nodeIDStrings = []string{"ns=3;i=1003"}
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:       endpoint,
				Username:       "",
				Password:       "",
				NodeIDs:        parsedNodeIDs,
				SecurityMode:   "None",
				SecurityPolicy: "None",
			}

			// Attempt to connect
			err := input.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			serverInformation, err := input.GetOPCUAServerInformation(ctx)
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Printf("Server Information: \n")
			GinkgoWriter.Printf("ManufacturerName: %s\n", serverInformation.ManufacturerName)
			GinkgoWriter.Printf("ProductName: %s\n", serverInformation.ProductName)
			GinkgoWriter.Printf("SoftwareVersion: %s\n", serverInformation.SoftwareVersion)

			// Close connection
			if input.Client != nil {
				err = input.Close(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Describe("YAML Configuration", func() {

		When("using a yaml and stream builder", func() {

			It("should subscribe to all nodes and receive data", func() {

				// Create a new stream builder
				builder := service.NewStreamBuilder()

				// Create a new stream
				err := builder.AddInputYAML(fmt.Sprintf(`
opcua:
  endpoint: "%s"
  username: ""
  password: ""
  subscribeEnabled: true
  useHeartbeat: true
  nodeIDs:
    - i=84
`, endpoint))

				Expect(err).NotTo(HaveOccurred())

				err = builder.SetLoggerYAML(`level: off`)
				Expect(err).NotTo(HaveOccurred())

				err = builder.SetTracerYAML(`type: none`)
				Expect(err).NotTo(HaveOccurred())

				// Add a total message count consumer
				var count int64
				err = builder.AddConsumerFunc(func(c context.Context, m *service.Message) error {
					atomic.AddInt64(&count, 1)
					return err
				})

				stream, err := builder.Build()
				Expect(err).NotTo(HaveOccurred())

				timeout := time.Second * 120

				// Run the stream
				ctx, cncl := context.WithTimeout(context.Background(), timeout)
				go func() {
					_ = stream.Run(ctx)
				}()

				// Check if we received any messages continuously
				Eventually(
					func() int64 {
						return atomic.LoadInt64(&count)
					}, timeout).Should(BeNumerically(">", int64(0)))

				cncl()

			})
		})

	})

	Describe("Insecure (None/None) Connect", func() {

		var endpoint string

		It("should read data correctly", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var nodeIDStrings = []string{"ns=3;i=1003"}
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:       endpoint,
				Username:       "",
				Password:       "",
				NodeIDs:        parsedNodeIDs,
				SecurityMode:   "None",
				SecurityPolicy: "None",
			}

			// Attempt to connect
			err := input.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(messageBatch).To(HaveLen(1))

			for _, message := range messageBatch {
				message, err := message.AsStructuredMut()
				Expect(err).NotTo(HaveOccurred())

				var exampleNumber json.Number = "22.565684"
				Expect(message).To(BeAssignableToTypeOf(exampleNumber))
				GinkgoWriter.Printf("Received message: %+v\n", message)
			}

			// Close connection
			if input.Client != nil {
				err = input.Close(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Describe("Secure (SignAndEncrypt/Basic256Sha256) Connect", func() {

		It("should read data correctly", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var nodeIDStrings = []string{"ns=3;i=1003"}
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:       endpoint,
				Username:       "",
				Password:       "",
				NodeIDs:        parsedNodeIDs,
				SecurityMode:   "SignAndEncrypt",
				SecurityPolicy: "Basic256Sha256",
			}

			// Attempt to connect
			err := input.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(messageBatch).To(HaveLen(1))

			for _, message := range messageBatch {
				message, err := message.AsStructuredMut()
				Expect(err).NotTo(HaveOccurred())

				var exampleNumber json.Number = "22.565684"
				Expect(message).To(BeAssignableToTypeOf(exampleNumber))
				GinkgoWriter.Printf("Received message: %+v\n", message)
			}

			// Close connection
			if input.Client != nil {
				err = input.Close(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

})

var _ = Describe("Test Against Microsoft OPC UA simulator", Serial, func() {

	BeforeEach(func() {
		testActivated := os.Getenv("TEST_OPCUA_SIMULATOR")

		// Check if environment variables are set
		if testActivated == "" {
			Skip("Skipping unit tests against simulator: TEST_OPCUA_SIMULATOR not set")
			return
		}
	})

	Describe("Connect Anonymous", func() {
		It("should connect anonymously", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var nodeIDStrings = []string{"ns=3;s=Basic"}

			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:         "opc.tcp://localhost:50000",
				Username:         "",
				Password:         "",
				NodeIDs:          parsedNodeIDs,
				SubscribeEnabled: false,
			}

			// Attempt to connect
			err := input.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(messageBatch).To(HaveLen(4))

			// Close connection
			if input.Client != nil {
				err = input.Close(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Describe("Connect with Username and Password", func() {
		Context("when using invalid credentials", func() {
			It("should fail to connect", func() {
				Skip("Skipping due to OPC-UA simulator limitations on logging in multiple times")
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				input := &OPCUAInput{
					Endpoint: "opc.tcp://localhost:50000",
					Username: "sysadmin_bad", // Incorrect username and password
					Password: "demo",
					NodeIDs:  nil,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).To(HaveOccurred())

				// Close connection
				if input.Client != nil {
					err := input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		Context("when using valid credentials", func() {
			It("should successfully connect", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				input := &OPCUAInput{
					Endpoint: "opc.tcp://localhost:50000",
					Username: "sysadmin", // Correct username and password
					Password: "demo",
					NodeIDs:  nil,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Close connection
				if input.Client != nil {
					err := input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
	})

	Describe("Subscribe", func() {
		Context("when connecting to subscribe to Fast data changes", func() {
			It("should connect and receive data changes", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=3;s=Fast"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				// ns=3;s=Fast consists out of 6 nodes
				// FastUInt1 - 5 and BadFastUInt1
				// BadFastUInt1 can sometimes be null, and then it will not report anything
				// Therefore, the expected messageBatch is between 5 and 6
				// However, sometimes the OPC UA server sends back the values for multiple seconds in the same batch, so it could also be 10 or 12
				Expect(len(messageBatch)).To(BeNumerically(">=", 5))

				for _, message := range messageBatch {
					message, err := message.AsStructuredMut()
					Expect(err).NotTo(HaveOccurred())
					Expect(message).To(BeAssignableToTypeOf(json.Number("22.565684")))
					//GinkgoT().Log("Received message: ", message)
				}

				// Close connection
				if input.Client != nil {
					err := input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		Context("when connecting to subscribe to Boolean with Properties", func() {
			It("should connect and confirm properties are not browsed by default", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=6;s=DataAccess_AnalogType_Byte"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(messageBatch)).To(Equal(1))

				for _, message := range messageBatch {
					message, err := message.AsStructuredMut()
					Expect(err).NotTo(HaveOccurred())
					Expect(message).To(Equal(json.Number("0")))
					//GinkgoT().Log("Received message: ", message)
				}

				// Close connection
				if input.Client != nil {
					err := input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
	})

	Describe("Subscribe to different datatypes", func() {
		When("Subscribing to AnalogTypes (simple datatypes)", func() {
			It("should connect and subscribe to AnalogTypes", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{
					"ns=6;s=DataAccess_AnalogType_Byte",
					"ns=6;s=DataAccess_AnalogType_Double",
					"ns=6;s=DataAccess_AnalogType_Float",
					"ns=6;s=DataAccess_AnalogType_Int16",
					"ns=6;s=DataAccess_AnalogType_Int32",
					"ns=6;s=DataAccess_AnalogType_Int64",
					"ns=6;s=DataAccess_AnalogType_SByte",
					"ns=6;s=DataAccess_AnalogType_UInt16",
					"ns=6;s=DataAccess_AnalogType_UInt32",
					"ns=6;s=DataAccess_AnalogType_UInt64",
				}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(Equal(10))

				for _, message := range messageBatch {
					message, err := message.AsStructuredMut()
					Expect(err).NotTo(HaveOccurred())
					Expect(message).To(BeAssignableToTypeOf(json.Number("22.565684")))
					//GinkgoT().Log("Received message: ", message)
				}

				// Close connection
				if input.Client != nil {
					err := input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to null datatype", func() {
			It("should not subscribe to null datatype", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=6;s=DataAccess_DataItem_Null"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(Equal(0)) // should never subscribe to null datatype

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to AnalogTypeArray", func() {
			It("should connect and subscribe to AnalogTypeArray and validate data types", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=6;s=DataAccess_AnalogType_Array"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(messageBatch)).To(Equal(26)) // Adjust the expected number of items as necessary

				for _, message := range messageBatch {
					messageParsed, err := message.AsStructuredMut()
					if err != nil {
						// This might happen if an empty string is returned from OPC-UA
						continue
					}

					opcuapath, found := message.MetaGet("opcua_path")
					Expect(found).To(BeTrue(), "Could not find opcua_path")

					// Determine the data type from the OPC UA path
					dataType := strings.Split(opcuapath, "_")[5] // This will extract the data type part of the OPC UA path

					// Check if the data type is an array and handle accordingly
					if strings.HasSuffix(dataType, "Array") {
						dataTypeOfArray := strings.Split(opcuapath, "_")[6]

						// Handle array data types
						switch dataTypeOfArray {
						case "Duration", "Guid", "LocaleId", "Boolean", "LocalizedText", "NodeId", "QualifiedName", "UtcTime", "DateTime", "Double", "Enumeration", "Float", "Int16", "Int32", "Int64", "Integer", "Number", "SByte", "StatusCode", "String", "UInt16", "UInt32", "UInt64", "UInteger", "Variant", "XmlElement", "ByteString":
							// Check if the messageParsed is of type slice (array)
							messageParsedArray, ok := messageParsed.([]interface{})
							Expect(ok).To(BeTrue(), fmt.Sprintf("Expected messageParsed to be an array, but got %T: %s : %s", messageParsed, opcuapath, messageParsed))

							for _, item := range messageParsedArray {
								// Here, use the checkDatatypeOfOPCUATag function adapted for Ginkgo
								checkDatatypeOfOPCUATag(dataTypeOfArray, item, opcuapath)
							}
						case "Byte":
							// Here too, use the checkDatatypeOfOPCUATag function adapted for Ginkgo
							checkDatatypeOfOPCUATag("ByteArray", messageParsed, opcuapath)
						default:
							Fail(fmt.Sprintf("Unsupported array data type in OPC UA path: %s:%s", dataType, opcuapath))
						}
					} else {
						Fail(fmt.Sprintf("Received non-array: %s", opcuapath))
					}
				}

				// Close connection
				if input.Client != nil {
					err := input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to DataItem", func() {
			It("should subscribe to all non-null datatype values", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=6;s=DataAccess_DataItem"} // Subscribes to all values with non-null datatype.
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(messageBatch)).To(Equal(23)) // these are theoretically >30, but most of them are null, so the browse function ignores them

				for _, message := range messageBatch {
					messageParsed, err := message.AsStructuredMut()
					if err != nil {
						// This might happen if an empty string is returned from OPC-UA
						continue
					}

					opcuapath, found := message.MetaGet("opcua_path")
					Expect(found).To(BeTrue(), "Could not find opcua_path")

					dataType := strings.Split(opcuapath, "_")[5] // Extracts the data type part of the OPC UA path.

					// Check the data type of the message.
					checkDatatypeOfOPCUATag(dataType, messageParsed, opcuapath)
				}

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Testing for failed node crash", func() {
			It("should not crash on failed nodes", func() {
				// https://github.com/united-manufacturing-hub/MgmtIssues/issues/1088
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{
					"ns=3;s=Fast",
					"ns=3;s=Slow",
				}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)
				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(messageBatch).NotTo(BeEmpty())

				for _, message := range messageBatch {
					_, err := message.AsStructured()
					Expect(err).NotTo(HaveOccurred())
				}

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to Scalar Arrays", func() {
			It("should subscribe to all scalar array values with non-null data types", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				nodeIDStrings := []string{
					"ns=6;s=Scalar_Static_Arrays_Boolean",
					"ns=6;s=Scalar_Static_Arrays_Byte",
					"ns=6;s=Scalar_Static_Arrays_ByteString",
					"ns=6;s=Scalar_Static_Arrays_DateTime",
					"ns=6;s=Scalar_Static_Arrays_Double",
					"ns=6;s=Scalar_Static_Arrays_Duration",
					"ns=6;s=Scalar_Static_Arrays_Float",
					"ns=6;s=Scalar_Static_Arrays_Guid",
					"ns=6;s=Scalar_Static_Arrays_Int16",
					"ns=6;s=Scalar_Static_Arrays_Int32",
					"ns=6;s=Scalar_Static_Arrays_Int64",
					"ns=6;s=Scalar_Static_Arrays_Integer",
					"ns=6;s=Scalar_Static_Arrays_LocaleId",
					"ns=6;s=Scalar_Static_Arrays_LocalizedText",
					"ns=6;s=Scalar_Static_Arrays_NodeId",
					"ns=6;s=Scalar_Static_Arrays_Number",
					"ns=6;s=Scalar_Static_Arrays_QualifiedName",
					"ns=6;s=Scalar_Static_Arrays_SByte",
					"ns=6;s=Scalar_Static_Arrays_String",
					"ns=6;s=Scalar_Static_Arrays_UInt16",
					"ns=6;s=Scalar_Static_Arrays_UInt32",
					"ns=6;s=Scalar_Static_Arrays_UInt64",
					"ns=6;s=Scalar_Static_Arrays_UInteger",
					"ns=6;s=Scalar_Static_Arrays_UtcTime",
					// "ns=6;s=Scalar_Static_Arrays_Variant", // Excluded due to library support issues
					"ns=6;s=Scalar_Static_Arrays_XmlElement",
				}

				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(messageBatch)).To(Equal(25))

				for _, message := range messageBatch {
					messageParsed, err := message.AsStructuredMut()
					if err != nil {
						// This might happen if an empty string is returned from OPC-UA
						continue
					}
					opcuapath, found := message.MetaGet("opcua_path")
					Expect(found).To(BeTrue(), "Could not find opcua_path")

					// Determine the data type from the OPC UA path
					dataType := strings.Split(opcuapath, "_")[5] // Extracts the data type part
					//GinkgoT().Log(dataType)

					if strings.HasSuffix(dataType, "Arrays") {
						dataTypeOfArray := strings.Split(opcuapath, "_")[6]
						//GinkgoT().Log(dataTypeOfArray)

						// Handle array data types
						switch dataTypeOfArray {
						case "Duration", "Guid", "LocaleId", "Boolean", "LocalizedText", "NodeId", "QualifiedName", "UtcTime", "DateTime", "Double", "Enumeration", "Float", "Int16", "Int32", "Int64", "SByte", "StatusCode", "String", "UInt16", "UInt32", "UInt64", "XmlElement", "ByteString":
							messageParsedArray, ok := messageParsed.([]interface{})
							Expect(ok).To(BeTrue(), fmt.Sprintf("Expected messageParsed to be an array, but got %T: %s : %s", messageParsed, opcuapath, messageParsed))

							for _, item := range messageParsedArray {
								// Use the adapted checkDatatypeOfOPCUATag function for Ginkgo
								checkDatatypeOfOPCUATag(dataTypeOfArray, item, opcuapath)
							}
						case "Byte", "Integer", "Number", "Variant", "UInteger": // Handling specific or unsupported types if necessary
							GinkgoT().Logf("Special handling or unsupported array data type in OPC UA path: %s:%s", dataType, opcuapath)
						default:
							Fail(fmt.Sprintf("Unsupported array data type in OPC UA path: %s:%s", dataType, opcuapath))
						}
					} else {
						Fail(fmt.Sprintf("Received non-array: %s", opcuapath))
					}
				}

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to the entire simulator", func() {
			It("does not fail", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=3;s=OpcPlc"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: false,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(BeNumerically(">=", 125))

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to everything", func() {
			It("does not fail", func() {
				Skip("This might take too long...")

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				nodeIDStrings := []string{"i=84"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(Equal(25))

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to everything", func() {
			It("does not fail", func() {
				Skip("This might take too long...")

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				nodeIDStrings := []string{"i=84"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: true,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(Equal(25))

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to Anomaly", func() {
			It("does not fail", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=3;s=Anomaly"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: false,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(Equal(4))

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to Basic", func() {
			It("does not fail", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=3;s=Basic"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: false,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(Equal(4))

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to Deterministic GUID", func() {
			It("does not fail", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=3;s=Deterministic GUID"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: false,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(Equal(5))

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to Fast", func() {
			It("does not fail", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=3;s=Fast"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: false,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(BeNumerically(">=", 5))

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to Slow", func() {
			It("does not fail", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=3;s=Slow"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: false,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(BeNumerically(">=", 100))

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

		When("Subscribing to Special", func() {
			It("does not fail", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				nodeIDStrings := []string{"ns=3;s=Special"}
				parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

				input := &OPCUAInput{
					Endpoint:         "opc.tcp://localhost:50000",
					Username:         "",
					Password:         "",
					NodeIDs:          parsedNodeIDs,
					SubscribeEnabled: false,
				}

				// Attempt to connect
				err := input.Connect(ctx)
				Expect(err).NotTo(HaveOccurred())

				messageBatch, _, err := input.ReadBatch(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(messageBatch)).To(Equal(7))

				// Close connection
				if input.Client != nil {
					err = input.Close(ctx)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})

	})

	Describe("metadata", func() {
		It("should create a proper opcua_tag_group and opcua_tag_name and opcua_tag_type", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var nodeIDStrings = []string{"ns=3;s=OpcPlc"}

			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:         "opc.tcp://localhost:50000",
				Username:         "",
				Password:         "",
				NodeIDs:          parsedNodeIDs,
				SubscribeEnabled: false,
			}

			// Attempt to connect
			err := input.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			// for each
			for _, message := range messageBatch {
				opcuaTagPath, err := message.MetaGet("opcua_tag_path")
				Expect(err).To(BeTrue(), "Could not find opcua_tag_path")
				GinkgoT().Log("opcua_tag_path: ", opcuaTagPath)

				opcuaTagGroup, err := message.MetaGet("opcua_tag_group")
				Expect(err).To(BeTrue(), "Could not find opcua_tag_group")
				GinkgoT().Log("opcua_tag_group: ", opcuaTagGroup)

				opcuaTagName, err := message.MetaGet("opcua_tag_name")
				Expect(err).To(BeTrue(), "Could not find opcua_tag_name")
				GinkgoT().Log("opcua_tag_name: ", opcuaTagName)

				opcuaTagType, err := message.MetaGet("opcua_tag_type")
				Expect(err).To(BeTrue(), "Could not find opcua_tag_type")
				GinkgoT().Log("opcua_tag_type: ", opcuaTagType)

				if opcuaTagPath == "StepUp" {
					Expect(opcuaTagGroup).To(Equal("OpcPlc.Telemetry.Basic"))
					Expect(opcuaTagName).To(Equal("StepUp"))
					Expect(opcuaTagType).To(Equal("number"))
				}
			}

			// Close connection
			if input.Client != nil {
				err = input.Close(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	When("Subscribing to slow values", func() {
		It("keeps sending data at least once every 10 seconds", func() {
			Skip("slow test")
			ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel1()

			nodeIDStrings := []string{"ns=3;s=SlowUInt1"}
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:         "opc.tcp://localhost:50000",
				Username:         "",
				Password:         "",
				NodeIDs:          parsedNodeIDs,
				SubscribeEnabled: true,
			}

			// Attempt to connect
			err := input.Connect(ctx1)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx1)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(1))

			time.Sleep(15 * time.Second)

			ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel2()

			messageBatch, _, err = input.ReadBatch(ctx2)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(1))

			// Close connection
			if input.Client != nil {
				ctxEnd, cancelEnd := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancelEnd()
				err = input.Close(ctxEnd)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	When("Enabling sendHeartbeat", func() {
		It("sends the heartbeat", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var nodeIDStrings []string
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:         "opc.tcp://localhost:50000",
				Username:         "",
				Password:         "",
				NodeIDs:          parsedNodeIDs,
				SubscribeEnabled: true,
				UseHeartbeat:     true,
				HeartbeatNodeId:  ua.NewNumericNodeID(0, 2258),
			}

			// Attempt to connect
			err := input.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(1))

			// Close connection
			if input.Client != nil {
				err = input.Close(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("sends the heartbeat when manual subscribe to i=2258", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			nodeIDStrings := []string{"i=2258"}
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:         "opc.tcp://localhost:50000",
				Username:         "",
				Password:         "",
				NodeIDs:          parsedNodeIDs,
				SubscribeEnabled: true,
				UseHeartbeat:     true,
				HeartbeatNodeId:  ua.NewNumericNodeID(0, 2258),
			}

			// Attempt to connect
			err := input.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(2))

			// Close connection
			if input.Client != nil {
				err = input.Close(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("does not disconnect if the heartbeat comes in in regular intervals", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var nodeIDStrings []string
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:         "opc.tcp://localhost:50000",
				Username:         "",
				Password:         "",
				NodeIDs:          parsedNodeIDs,
				SubscribeEnabled: true,
				UseHeartbeat:     true,
				HeartbeatNodeId:  ua.NewNumericNodeID(0, 2258),
			}

			// Attempt to connect
			err := input.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(1))

			time.Sleep(5 * time.Second)

			messageBatch, _, err = input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(1))

			time.Sleep(5 * time.Second)

			messageBatch, _, err = input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(1))

			// Close connection
			if input.Client != nil {
				err = input.Close(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("does disconnect if the heartbeat does not come in regular intervals", func() {

			var nodeIDStrings []string
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:         "opc.tcp://localhost:50000",
				Username:         "",
				Password:         "",
				NodeIDs:          parsedNodeIDs,
				SubscribeEnabled: true,
				UseHeartbeat:     true,
				HeartbeatNodeId:  ua.NewNumericNodeID(0, 2259), // 2259 is State, which will not change
			}

			// Attempt to connect
			ctx1, cancel1 := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel1()
			err := input.Connect(ctx1)
			Expect(err).NotTo(HaveOccurred())

			ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel2()
			messageBatch, _, err := input.ReadBatch(ctx2)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(1))

			time.Sleep(5 * time.Second)

			ctx3, cancel3 := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel3()
			messageBatch, _, err = input.ReadBatch(ctx3)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(0))

			time.Sleep(10 * time.Second)

			ctx4, cancel4 := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel4()
			messageBatch, _, err = input.ReadBatch(ctx4)
			// Expect err to be service.ErrNotConnected
			Expect(err).To(Equal(service.ErrNotConnected))

			// Close connection
			if input.Client != nil {
				ctx6, cancel6 := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel6()
				err = input.Close(ctx6)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})

var _ = Describe("Test Against Softing OPC DataFeed", Serial, func() {
	When("Subscribing to server without discovery urls", func() {
		It("does successfully connects", func() {
			Skip("not implemented in CI pipeline")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			nodeIDStrings := []string{"ns=3;s=Siemens_1"}
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:         "opc.tcp://10.13.37.125:4998",
				Username:         "",
				Password:         "",
				NodeIDs:          parsedNodeIDs,
				SubscribeEnabled: false,
			}

			// Attempt to connect
			err := input.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(messageBatch)).To(Equal(1))

			// Close connection
			if input.Client != nil {
				err = input.Close(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
	When("Subscribing to softing and manually adjusting local item", func() {
		It("does successfully reports a data change", func() {
			// This requires manual intervention and a manual change of localitem during this test
			Skip("not implemented in CI pipeline")
			ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel1()

			nodeIDStrings := []string{"ns=3;s=Local Items.test"}
			parsedNodeIDs := ParseNodeIDs(nodeIDStrings)

			input := &OPCUAInput{
				Endpoint:         "opc.tcp://10.13.37.125:4998",
				Username:         "",
				Password:         "",
				NodeIDs:          parsedNodeIDs,
				SubscribeEnabled: true,
			}

			// Attempt to connect
			err := input.Connect(ctx1)
			Expect(err).NotTo(HaveOccurred())

			messageBatch, _, err := input.ReadBatch(ctx1)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(messageBatch)).To(Equal(1))

			ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel2()

			messageBatch, _, err = input.ReadBatch(ctx2)
			Expect(err).To(Equal(context.DeadlineExceeded)) // there should be no data change
			Expect(len(messageBatch)).To(Equal(0))

			ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel3()

			// at this point of time local items is changed
			GinkgoWriter.Println("Change local item now")

			messageBatch, _, err = input.ReadBatch(ctx3)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(messageBatch)).To(Equal(1))

			// Close connection
			if input.Client != nil {
				ctxEnd, cancelEnd := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancelEnd()
				err = input.Close(ctxEnd)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})

func checkDatatypeOfOPCUATag(dataType string, messageParsed interface{}, opcuapath string) {
	//GinkgoT().Logf("%s, %+v, %s", dataType, messageParsed, opcuapath)
	switch dataType {
	case "Boolean":
		// Assignable to bool
		Expect(messageParsed).To(BeAssignableToTypeOf(true), "DataType check for Boolean failed")
		//GinkgoT().Log("Received Boolean message: ", messageParsed)

	case "Byte", "Double", "Enumeration", "Float", "Int16", "Int32", "Int64", "Integer", "Number", "SByte", "StatusCode", "UInt16", "UInt32", "UInt64", "UInteger", "Duration":
		// Assignable to json.number
		Expect(messageParsed).To(BeAssignableToTypeOf(json.Number("")), "DataType check for Byte failed")
		//GinkgoT().Log("Received message: ", dataType, messageParsed)

	case "DateTime", "NodeId", "String", "ByteArray", "ByteString", "LocaleId", "UtcTime", "XmlElement":
		// Assignable to string
		Expect(messageParsed).To(BeAssignableToTypeOf("12fff3"), "DataType check for DateTime failed")
		//GinkgoT().Log("Received DateTime message: ", messageParsed)
	case "ExpandedNodeId":
		// ExpandedNodeId is expected to be a map[string]interface{}
		parsedMap, ok := messageParsed.(map[string]interface{})
		Expect(ok).To(BeTrue(), "Expected messageParsed to be of type map[string]interface{}")

		// Define the expected keys for ExpandedNodeId
		expectedKeys := []string{"NamespaceURI", "NodeID", "ServerIndex"}
		for _, key := range expectedKeys {
			_, exists := parsedMap[key]
			Expect(exists).To(BeTrue(), fmt.Sprintf("Expected key %s missing in messageParsed", key))
		}

		//GinkgoT().Log("Received ExpandedNodeId message: ", messageParsed)

	case "Guid", "LocalizedText", "QualifiedName":
		// These types are expected to be map[string]interface{} with specific keys
		parsedMap, ok := messageParsed.(map[string]interface{})
		Expect(ok).To(BeTrue(), fmt.Sprintf("Expected messageParsed to be of type map[string]interface{} for %s", dataType))

		var expectedKeys []string
		switch dataType {
		case "Guid":
			expectedKeys = []string{"Data1", "Data2", "Data3", "Data4"}
		case "LocalizedText":
			expectedKeys = []string{"EncodingMask", "Locale", "Text"}
		case "QualifiedName":
			expectedKeys = []string{"NamespaceIndex", "Name"}
		}

		for _, key := range expectedKeys {
			_, exists := parsedMap[key]
			Expect(exists).To(BeTrue(), fmt.Sprintf("Expected key %s missing in messageParsed for %s", key, dataType))
		}

		//GinkgoT().Logf("Received %s message: %+v", dataType, messageParsed)

	case "Variant":
		// Variant is expected to be a map[string]interface{}, but without specified keys
		_, ok := messageParsed.(map[string]interface{})
		Expect(ok).To(BeTrue(), "Expected messageParsed to be of type map[string]interface{} for Variant")
		//GinkgoT().Log("Received Variant message: ", messageParsed)

	default:
		Fail(fmt.Sprintf("Unsupported data type in OPC UA path: %s:%s", dataType, opcuapath))
	}
}
