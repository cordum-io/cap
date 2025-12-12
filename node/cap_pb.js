/*eslint-disable block-scoped-var, id-length, no-control-regex, no-magic-numbers, no-prototype-builtins, no-redeclare, no-shadow, no-var, sort-vars*/
"use strict";

var $protobuf = require("protobufjs/minimal");

// Common aliases
var $Reader = $protobuf.Reader, $Writer = $protobuf.Writer, $util = $protobuf.util;

// Exported root namespace
var $root = $protobuf.roots["coretex.agent.v1"] || ($protobuf.roots["coretex.agent.v1"] = {});

$root.coretex = (function() {

    /**
     * Namespace coretex.
     * @exports coretex
     * @namespace
     */
    var coretex = {};

    coretex.agent = (function() {

        /**
         * Namespace agent.
         * @memberof coretex
         * @namespace
         */
        var agent = {};

        agent.v1 = (function() {

            /**
             * Namespace v1.
             * @memberof coretex.agent
             * @namespace
             */
            var v1 = {};

            v1.SystemAlert = (function() {

                /**
                 * Properties of a SystemAlert.
                 * @memberof coretex.agent.v1
                 * @interface ISystemAlert
                 * @property {string|null} [level] SystemAlert level
                 * @property {string|null} [message] SystemAlert message
                 * @property {string|null} [component] SystemAlert component
                 * @property {string|null} [code] SystemAlert code
                 */

                /**
                 * Constructs a new SystemAlert.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a SystemAlert.
                 * @implements ISystemAlert
                 * @constructor
                 * @param {coretex.agent.v1.ISystemAlert=} [properties] Properties to set
                 */
                function SystemAlert(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * SystemAlert level.
                 * @member {string} level
                 * @memberof coretex.agent.v1.SystemAlert
                 * @instance
                 */
                SystemAlert.prototype.level = "";

                /**
                 * SystemAlert message.
                 * @member {string} message
                 * @memberof coretex.agent.v1.SystemAlert
                 * @instance
                 */
                SystemAlert.prototype.message = "";

                /**
                 * SystemAlert component.
                 * @member {string} component
                 * @memberof coretex.agent.v1.SystemAlert
                 * @instance
                 */
                SystemAlert.prototype.component = "";

                /**
                 * SystemAlert code.
                 * @member {string} code
                 * @memberof coretex.agent.v1.SystemAlert
                 * @instance
                 */
                SystemAlert.prototype.code = "";

                /**
                 * Creates a new SystemAlert instance using the specified properties.
                 * @function create
                 * @memberof coretex.agent.v1.SystemAlert
                 * @static
                 * @param {coretex.agent.v1.ISystemAlert=} [properties] Properties to set
                 * @returns {coretex.agent.v1.SystemAlert} SystemAlert instance
                 */
                SystemAlert.create = function create(properties) {
                    return new SystemAlert(properties);
                };

                /**
                 * Encodes the specified SystemAlert message. Does not implicitly {@link coretex.agent.v1.SystemAlert.verify|verify} messages.
                 * @function encode
                 * @memberof coretex.agent.v1.SystemAlert
                 * @static
                 * @param {coretex.agent.v1.ISystemAlert} message SystemAlert message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                SystemAlert.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.level != null && Object.hasOwnProperty.call(message, "level"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.level);
                    if (message.message != null && Object.hasOwnProperty.call(message, "message"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.message);
                    if (message.component != null && Object.hasOwnProperty.call(message, "component"))
                        writer.uint32(/* id 3, wireType 2 =*/26).string(message.component);
                    if (message.code != null && Object.hasOwnProperty.call(message, "code"))
                        writer.uint32(/* id 4, wireType 2 =*/34).string(message.code);
                    return writer;
                };

                /**
                 * Encodes the specified SystemAlert message, length delimited. Does not implicitly {@link coretex.agent.v1.SystemAlert.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof coretex.agent.v1.SystemAlert
                 * @static
                 * @param {coretex.agent.v1.ISystemAlert} message SystemAlert message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                SystemAlert.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a SystemAlert message from the specified reader or buffer.
                 * @function decode
                 * @memberof coretex.agent.v1.SystemAlert
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {coretex.agent.v1.SystemAlert} SystemAlert
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                SystemAlert.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.coretex.agent.v1.SystemAlert();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.level = reader.string();
                                break;
                            }
                        case 2: {
                                message.message = reader.string();
                                break;
                            }
                        case 3: {
                                message.component = reader.string();
                                break;
                            }
                        case 4: {
                                message.code = reader.string();
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a SystemAlert message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof coretex.agent.v1.SystemAlert
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {coretex.agent.v1.SystemAlert} SystemAlert
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                SystemAlert.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a SystemAlert message.
                 * @function verify
                 * @memberof coretex.agent.v1.SystemAlert
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                SystemAlert.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.level != null && message.hasOwnProperty("level"))
                        if (!$util.isString(message.level))
                            return "level: string expected";
                    if (message.message != null && message.hasOwnProperty("message"))
                        if (!$util.isString(message.message))
                            return "message: string expected";
                    if (message.component != null && message.hasOwnProperty("component"))
                        if (!$util.isString(message.component))
                            return "component: string expected";
                    if (message.code != null && message.hasOwnProperty("code"))
                        if (!$util.isString(message.code))
                            return "code: string expected";
                    return null;
                };

                /**
                 * Creates a SystemAlert message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof coretex.agent.v1.SystemAlert
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {coretex.agent.v1.SystemAlert} SystemAlert
                 */
                SystemAlert.fromObject = function fromObject(object) {
                    if (object instanceof $root.coretex.agent.v1.SystemAlert)
                        return object;
                    var message = new $root.coretex.agent.v1.SystemAlert();
                    if (object.level != null)
                        message.level = String(object.level);
                    if (object.message != null)
                        message.message = String(object.message);
                    if (object.component != null)
                        message.component = String(object.component);
                    if (object.code != null)
                        message.code = String(object.code);
                    return message;
                };

                /**
                 * Creates a plain object from a SystemAlert message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof coretex.agent.v1.SystemAlert
                 * @static
                 * @param {coretex.agent.v1.SystemAlert} message SystemAlert
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                SystemAlert.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.defaults) {
                        object.level = "";
                        object.message = "";
                        object.component = "";
                        object.code = "";
                    }
                    if (message.level != null && message.hasOwnProperty("level"))
                        object.level = message.level;
                    if (message.message != null && message.hasOwnProperty("message"))
                        object.message = message.message;
                    if (message.component != null && message.hasOwnProperty("component"))
                        object.component = message.component;
                    if (message.code != null && message.hasOwnProperty("code"))
                        object.code = message.code;
                    return object;
                };

                /**
                 * Converts this SystemAlert to JSON.
                 * @function toJSON
                 * @memberof coretex.agent.v1.SystemAlert
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                SystemAlert.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for SystemAlert
                 * @function getTypeUrl
                 * @memberof coretex.agent.v1.SystemAlert
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                SystemAlert.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/coretex.agent.v1.SystemAlert";
                };

                return SystemAlert;
            })();

            v1.BusPacket = (function() {

                /**
                 * Properties of a BusPacket.
                 * @memberof coretex.agent.v1
                 * @interface IBusPacket
                 * @property {string|null} [traceId] BusPacket traceId
                 * @property {string|null} [senderId] BusPacket senderId
                 * @property {google.protobuf.ITimestamp|null} [createdAt] BusPacket createdAt
                 * @property {number|null} [protocolVersion] BusPacket protocolVersion
                 * @property {coretex.agent.v1.IJobRequest|null} [jobRequest] BusPacket jobRequest
                 * @property {coretex.agent.v1.IJobResult|null} [jobResult] BusPacket jobResult
                 * @property {coretex.agent.v1.IHeartbeat|null} [heartbeat] BusPacket heartbeat
                 * @property {coretex.agent.v1.ISystemAlert|null} [alert] BusPacket alert
                 * @property {Uint8Array|null} [signature] BusPacket signature
                 */

                /**
                 * Constructs a new BusPacket.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a BusPacket.
                 * @implements IBusPacket
                 * @constructor
                 * @param {coretex.agent.v1.IBusPacket=} [properties] Properties to set
                 */
                function BusPacket(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * BusPacket traceId.
                 * @member {string} traceId
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.traceId = "";

                /**
                 * BusPacket senderId.
                 * @member {string} senderId
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.senderId = "";

                /**
                 * BusPacket createdAt.
                 * @member {google.protobuf.ITimestamp|null|undefined} createdAt
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.createdAt = null;

                /**
                 * BusPacket protocolVersion.
                 * @member {number} protocolVersion
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.protocolVersion = 0;

                /**
                 * BusPacket jobRequest.
                 * @member {coretex.agent.v1.IJobRequest|null|undefined} jobRequest
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.jobRequest = null;

                /**
                 * BusPacket jobResult.
                 * @member {coretex.agent.v1.IJobResult|null|undefined} jobResult
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.jobResult = null;

                /**
                 * BusPacket heartbeat.
                 * @member {coretex.agent.v1.IHeartbeat|null|undefined} heartbeat
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.heartbeat = null;

                /**
                 * BusPacket alert.
                 * @member {coretex.agent.v1.ISystemAlert|null|undefined} alert
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.alert = null;

                /**
                 * BusPacket signature.
                 * @member {Uint8Array} signature
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.signature = $util.newBuffer([]);

                // OneOf field names bound to virtual getters and setters
                var $oneOfFields;

                /**
                 * BusPacket payload.
                 * @member {"jobRequest"|"jobResult"|"heartbeat"|"alert"|undefined} payload
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 */
                Object.defineProperty(BusPacket.prototype, "payload", {
                    get: $util.oneOfGetter($oneOfFields = ["jobRequest", "jobResult", "heartbeat", "alert"]),
                    set: $util.oneOfSetter($oneOfFields)
                });

                /**
                 * Creates a new BusPacket instance using the specified properties.
                 * @function create
                 * @memberof coretex.agent.v1.BusPacket
                 * @static
                 * @param {coretex.agent.v1.IBusPacket=} [properties] Properties to set
                 * @returns {coretex.agent.v1.BusPacket} BusPacket instance
                 */
                BusPacket.create = function create(properties) {
                    return new BusPacket(properties);
                };

                /**
                 * Encodes the specified BusPacket message. Does not implicitly {@link coretex.agent.v1.BusPacket.verify|verify} messages.
                 * @function encode
                 * @memberof coretex.agent.v1.BusPacket
                 * @static
                 * @param {coretex.agent.v1.IBusPacket} message BusPacket message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                BusPacket.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.traceId != null && Object.hasOwnProperty.call(message, "traceId"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.traceId);
                    if (message.senderId != null && Object.hasOwnProperty.call(message, "senderId"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.senderId);
                    if (message.createdAt != null && Object.hasOwnProperty.call(message, "createdAt"))
                        $root.google.protobuf.Timestamp.encode(message.createdAt, writer.uint32(/* id 3, wireType 2 =*/26).fork()).ldelim();
                    if (message.protocolVersion != null && Object.hasOwnProperty.call(message, "protocolVersion"))
                        writer.uint32(/* id 4, wireType 0 =*/32).int32(message.protocolVersion);
                    if (message.jobRequest != null && Object.hasOwnProperty.call(message, "jobRequest"))
                        $root.coretex.agent.v1.JobRequest.encode(message.jobRequest, writer.uint32(/* id 10, wireType 2 =*/82).fork()).ldelim();
                    if (message.jobResult != null && Object.hasOwnProperty.call(message, "jobResult"))
                        $root.coretex.agent.v1.JobResult.encode(message.jobResult, writer.uint32(/* id 11, wireType 2 =*/90).fork()).ldelim();
                    if (message.heartbeat != null && Object.hasOwnProperty.call(message, "heartbeat"))
                        $root.coretex.agent.v1.Heartbeat.encode(message.heartbeat, writer.uint32(/* id 12, wireType 2 =*/98).fork()).ldelim();
                    if (message.alert != null && Object.hasOwnProperty.call(message, "alert"))
                        $root.coretex.agent.v1.SystemAlert.encode(message.alert, writer.uint32(/* id 13, wireType 2 =*/106).fork()).ldelim();
                    if (message.signature != null && Object.hasOwnProperty.call(message, "signature"))
                        writer.uint32(/* id 14, wireType 2 =*/114).bytes(message.signature);
                    return writer;
                };

                /**
                 * Encodes the specified BusPacket message, length delimited. Does not implicitly {@link coretex.agent.v1.BusPacket.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof coretex.agent.v1.BusPacket
                 * @static
                 * @param {coretex.agent.v1.IBusPacket} message BusPacket message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                BusPacket.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a BusPacket message from the specified reader or buffer.
                 * @function decode
                 * @memberof coretex.agent.v1.BusPacket
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {coretex.agent.v1.BusPacket} BusPacket
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                BusPacket.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.coretex.agent.v1.BusPacket();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.traceId = reader.string();
                                break;
                            }
                        case 2: {
                                message.senderId = reader.string();
                                break;
                            }
                        case 3: {
                                message.createdAt = $root.google.protobuf.Timestamp.decode(reader, reader.uint32());
                                break;
                            }
                        case 4: {
                                message.protocolVersion = reader.int32();
                                break;
                            }
                        case 10: {
                                message.jobRequest = $root.coretex.agent.v1.JobRequest.decode(reader, reader.uint32());
                                break;
                            }
                        case 11: {
                                message.jobResult = $root.coretex.agent.v1.JobResult.decode(reader, reader.uint32());
                                break;
                            }
                        case 12: {
                                message.heartbeat = $root.coretex.agent.v1.Heartbeat.decode(reader, reader.uint32());
                                break;
                            }
                        case 13: {
                                message.alert = $root.coretex.agent.v1.SystemAlert.decode(reader, reader.uint32());
                                break;
                            }
                        case 14: {
                                message.signature = reader.bytes();
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a BusPacket message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof coretex.agent.v1.BusPacket
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {coretex.agent.v1.BusPacket} BusPacket
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                BusPacket.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a BusPacket message.
                 * @function verify
                 * @memberof coretex.agent.v1.BusPacket
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                BusPacket.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    var properties = {};
                    if (message.traceId != null && message.hasOwnProperty("traceId"))
                        if (!$util.isString(message.traceId))
                            return "traceId: string expected";
                    if (message.senderId != null && message.hasOwnProperty("senderId"))
                        if (!$util.isString(message.senderId))
                            return "senderId: string expected";
                    if (message.createdAt != null && message.hasOwnProperty("createdAt")) {
                        var error = $root.google.protobuf.Timestamp.verify(message.createdAt);
                        if (error)
                            return "createdAt." + error;
                    }
                    if (message.protocolVersion != null && message.hasOwnProperty("protocolVersion"))
                        if (!$util.isInteger(message.protocolVersion))
                            return "protocolVersion: integer expected";
                    if (message.jobRequest != null && message.hasOwnProperty("jobRequest")) {
                        properties.payload = 1;
                        {
                            var error = $root.coretex.agent.v1.JobRequest.verify(message.jobRequest);
                            if (error)
                                return "jobRequest." + error;
                        }
                    }
                    if (message.jobResult != null && message.hasOwnProperty("jobResult")) {
                        if (properties.payload === 1)
                            return "payload: multiple values";
                        properties.payload = 1;
                        {
                            var error = $root.coretex.agent.v1.JobResult.verify(message.jobResult);
                            if (error)
                                return "jobResult." + error;
                        }
                    }
                    if (message.heartbeat != null && message.hasOwnProperty("heartbeat")) {
                        if (properties.payload === 1)
                            return "payload: multiple values";
                        properties.payload = 1;
                        {
                            var error = $root.coretex.agent.v1.Heartbeat.verify(message.heartbeat);
                            if (error)
                                return "heartbeat." + error;
                        }
                    }
                    if (message.alert != null && message.hasOwnProperty("alert")) {
                        if (properties.payload === 1)
                            return "payload: multiple values";
                        properties.payload = 1;
                        {
                            var error = $root.coretex.agent.v1.SystemAlert.verify(message.alert);
                            if (error)
                                return "alert." + error;
                        }
                    }
                    if (message.signature != null && message.hasOwnProperty("signature"))
                        if (!(message.signature && typeof message.signature.length === "number" || $util.isString(message.signature)))
                            return "signature: buffer expected";
                    return null;
                };

                /**
                 * Creates a BusPacket message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof coretex.agent.v1.BusPacket
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {coretex.agent.v1.BusPacket} BusPacket
                 */
                BusPacket.fromObject = function fromObject(object) {
                    if (object instanceof $root.coretex.agent.v1.BusPacket)
                        return object;
                    var message = new $root.coretex.agent.v1.BusPacket();
                    if (object.traceId != null)
                        message.traceId = String(object.traceId);
                    if (object.senderId != null)
                        message.senderId = String(object.senderId);
                    if (object.createdAt != null) {
                        if (typeof object.createdAt !== "object")
                            throw TypeError(".coretex.agent.v1.BusPacket.createdAt: object expected");
                        message.createdAt = $root.google.protobuf.Timestamp.fromObject(object.createdAt);
                    }
                    if (object.protocolVersion != null)
                        message.protocolVersion = object.protocolVersion | 0;
                    if (object.jobRequest != null) {
                        if (typeof object.jobRequest !== "object")
                            throw TypeError(".coretex.agent.v1.BusPacket.jobRequest: object expected");
                        message.jobRequest = $root.coretex.agent.v1.JobRequest.fromObject(object.jobRequest);
                    }
                    if (object.jobResult != null) {
                        if (typeof object.jobResult !== "object")
                            throw TypeError(".coretex.agent.v1.BusPacket.jobResult: object expected");
                        message.jobResult = $root.coretex.agent.v1.JobResult.fromObject(object.jobResult);
                    }
                    if (object.heartbeat != null) {
                        if (typeof object.heartbeat !== "object")
                            throw TypeError(".coretex.agent.v1.BusPacket.heartbeat: object expected");
                        message.heartbeat = $root.coretex.agent.v1.Heartbeat.fromObject(object.heartbeat);
                    }
                    if (object.alert != null) {
                        if (typeof object.alert !== "object")
                            throw TypeError(".coretex.agent.v1.BusPacket.alert: object expected");
                        message.alert = $root.coretex.agent.v1.SystemAlert.fromObject(object.alert);
                    }
                    if (object.signature != null)
                        if (typeof object.signature === "string")
                            $util.base64.decode(object.signature, message.signature = $util.newBuffer($util.base64.length(object.signature)), 0);
                        else if (object.signature.length >= 0)
                            message.signature = object.signature;
                    return message;
                };

                /**
                 * Creates a plain object from a BusPacket message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof coretex.agent.v1.BusPacket
                 * @static
                 * @param {coretex.agent.v1.BusPacket} message BusPacket
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                BusPacket.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.defaults) {
                        object.traceId = "";
                        object.senderId = "";
                        object.createdAt = null;
                        object.protocolVersion = 0;
                        if (options.bytes === String)
                            object.signature = "";
                        else {
                            object.signature = [];
                            if (options.bytes !== Array)
                                object.signature = $util.newBuffer(object.signature);
                        }
                    }
                    if (message.traceId != null && message.hasOwnProperty("traceId"))
                        object.traceId = message.traceId;
                    if (message.senderId != null && message.hasOwnProperty("senderId"))
                        object.senderId = message.senderId;
                    if (message.createdAt != null && message.hasOwnProperty("createdAt"))
                        object.createdAt = $root.google.protobuf.Timestamp.toObject(message.createdAt, options);
                    if (message.protocolVersion != null && message.hasOwnProperty("protocolVersion"))
                        object.protocolVersion = message.protocolVersion;
                    if (message.jobRequest != null && message.hasOwnProperty("jobRequest")) {
                        object.jobRequest = $root.coretex.agent.v1.JobRequest.toObject(message.jobRequest, options);
                        if (options.oneofs)
                            object.payload = "jobRequest";
                    }
                    if (message.jobResult != null && message.hasOwnProperty("jobResult")) {
                        object.jobResult = $root.coretex.agent.v1.JobResult.toObject(message.jobResult, options);
                        if (options.oneofs)
                            object.payload = "jobResult";
                    }
                    if (message.heartbeat != null && message.hasOwnProperty("heartbeat")) {
                        object.heartbeat = $root.coretex.agent.v1.Heartbeat.toObject(message.heartbeat, options);
                        if (options.oneofs)
                            object.payload = "heartbeat";
                    }
                    if (message.alert != null && message.hasOwnProperty("alert")) {
                        object.alert = $root.coretex.agent.v1.SystemAlert.toObject(message.alert, options);
                        if (options.oneofs)
                            object.payload = "alert";
                    }
                    if (message.signature != null && message.hasOwnProperty("signature"))
                        object.signature = options.bytes === String ? $util.base64.encode(message.signature, 0, message.signature.length) : options.bytes === Array ? Array.prototype.slice.call(message.signature) : message.signature;
                    return object;
                };

                /**
                 * Converts this BusPacket to JSON.
                 * @function toJSON
                 * @memberof coretex.agent.v1.BusPacket
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                BusPacket.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for BusPacket
                 * @function getTypeUrl
                 * @memberof coretex.agent.v1.BusPacket
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                BusPacket.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/coretex.agent.v1.BusPacket";
                };

                return BusPacket;
            })();

            /**
             * JobPriority enum.
             * @name coretex.agent.v1.JobPriority
             * @enum {number}
             * @property {number} JOB_PRIORITY_UNSPECIFIED=0 JOB_PRIORITY_UNSPECIFIED value
             * @property {number} JOB_PRIORITY_INTERACTIVE=1 JOB_PRIORITY_INTERACTIVE value
             * @property {number} JOB_PRIORITY_BATCH=2 JOB_PRIORITY_BATCH value
             * @property {number} JOB_PRIORITY_CRITICAL=3 JOB_PRIORITY_CRITICAL value
             */
            v1.JobPriority = (function() {
                var valuesById = {}, values = Object.create(valuesById);
                values[valuesById[0] = "JOB_PRIORITY_UNSPECIFIED"] = 0;
                values[valuesById[1] = "JOB_PRIORITY_INTERACTIVE"] = 1;
                values[valuesById[2] = "JOB_PRIORITY_BATCH"] = 2;
                values[valuesById[3] = "JOB_PRIORITY_CRITICAL"] = 3;
                return values;
            })();

            /**
             * JobStatus enum.
             * @name coretex.agent.v1.JobStatus
             * @enum {number}
             * @property {number} JOB_STATUS_UNSPECIFIED=0 JOB_STATUS_UNSPECIFIED value
             * @property {number} JOB_STATUS_PENDING=1 JOB_STATUS_PENDING value
             * @property {number} JOB_STATUS_SCHEDULED=2 JOB_STATUS_SCHEDULED value
             * @property {number} JOB_STATUS_DISPATCHED=3 JOB_STATUS_DISPATCHED value
             * @property {number} JOB_STATUS_RUNNING=4 JOB_STATUS_RUNNING value
             * @property {number} JOB_STATUS_SUCCEEDED=5 JOB_STATUS_SUCCEEDED value
             * @property {number} JOB_STATUS_FAILED=6 JOB_STATUS_FAILED value
             * @property {number} JOB_STATUS_CANCELLED=7 JOB_STATUS_CANCELLED value
             * @property {number} JOB_STATUS_DENIED=8 JOB_STATUS_DENIED value
             * @property {number} JOB_STATUS_TIMEOUT=9 JOB_STATUS_TIMEOUT value
             */
            v1.JobStatus = (function() {
                var valuesById = {}, values = Object.create(valuesById);
                values[valuesById[0] = "JOB_STATUS_UNSPECIFIED"] = 0;
                values[valuesById[1] = "JOB_STATUS_PENDING"] = 1;
                values[valuesById[2] = "JOB_STATUS_SCHEDULED"] = 2;
                values[valuesById[3] = "JOB_STATUS_DISPATCHED"] = 3;
                values[valuesById[4] = "JOB_STATUS_RUNNING"] = 4;
                values[valuesById[5] = "JOB_STATUS_SUCCEEDED"] = 5;
                values[valuesById[6] = "JOB_STATUS_FAILED"] = 6;
                values[valuesById[7] = "JOB_STATUS_CANCELLED"] = 7;
                values[valuesById[8] = "JOB_STATUS_DENIED"] = 8;
                values[valuesById[9] = "JOB_STATUS_TIMEOUT"] = 9;
                return values;
            })();

            v1.ContextHints = (function() {

                /**
                 * Properties of a ContextHints.
                 * @memberof coretex.agent.v1
                 * @interface IContextHints
                 * @property {number|null} [maxInputTokens] ContextHints maxInputTokens
                 * @property {boolean|null} [allowSummarization] ContextHints allowSummarization
                 * @property {boolean|null} [allowRetrieval] ContextHints allowRetrieval
                 * @property {Array.<string>|null} [tags] ContextHints tags
                 */

                /**
                 * Constructs a new ContextHints.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a ContextHints.
                 * @implements IContextHints
                 * @constructor
                 * @param {coretex.agent.v1.IContextHints=} [properties] Properties to set
                 */
                function ContextHints(properties) {
                    this.tags = [];
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * ContextHints maxInputTokens.
                 * @member {number} maxInputTokens
                 * @memberof coretex.agent.v1.ContextHints
                 * @instance
                 */
                ContextHints.prototype.maxInputTokens = 0;

                /**
                 * ContextHints allowSummarization.
                 * @member {boolean} allowSummarization
                 * @memberof coretex.agent.v1.ContextHints
                 * @instance
                 */
                ContextHints.prototype.allowSummarization = false;

                /**
                 * ContextHints allowRetrieval.
                 * @member {boolean} allowRetrieval
                 * @memberof coretex.agent.v1.ContextHints
                 * @instance
                 */
                ContextHints.prototype.allowRetrieval = false;

                /**
                 * ContextHints tags.
                 * @member {Array.<string>} tags
                 * @memberof coretex.agent.v1.ContextHints
                 * @instance
                 */
                ContextHints.prototype.tags = $util.emptyArray;

                /**
                 * Creates a new ContextHints instance using the specified properties.
                 * @function create
                 * @memberof coretex.agent.v1.ContextHints
                 * @static
                 * @param {coretex.agent.v1.IContextHints=} [properties] Properties to set
                 * @returns {coretex.agent.v1.ContextHints} ContextHints instance
                 */
                ContextHints.create = function create(properties) {
                    return new ContextHints(properties);
                };

                /**
                 * Encodes the specified ContextHints message. Does not implicitly {@link coretex.agent.v1.ContextHints.verify|verify} messages.
                 * @function encode
                 * @memberof coretex.agent.v1.ContextHints
                 * @static
                 * @param {coretex.agent.v1.IContextHints} message ContextHints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ContextHints.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.maxInputTokens != null && Object.hasOwnProperty.call(message, "maxInputTokens"))
                        writer.uint32(/* id 1, wireType 0 =*/8).int32(message.maxInputTokens);
                    if (message.allowSummarization != null && Object.hasOwnProperty.call(message, "allowSummarization"))
                        writer.uint32(/* id 2, wireType 0 =*/16).bool(message.allowSummarization);
                    if (message.allowRetrieval != null && Object.hasOwnProperty.call(message, "allowRetrieval"))
                        writer.uint32(/* id 3, wireType 0 =*/24).bool(message.allowRetrieval);
                    if (message.tags != null && message.tags.length)
                        for (var i = 0; i < message.tags.length; ++i)
                            writer.uint32(/* id 4, wireType 2 =*/34).string(message.tags[i]);
                    return writer;
                };

                /**
                 * Encodes the specified ContextHints message, length delimited. Does not implicitly {@link coretex.agent.v1.ContextHints.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof coretex.agent.v1.ContextHints
                 * @static
                 * @param {coretex.agent.v1.IContextHints} message ContextHints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ContextHints.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a ContextHints message from the specified reader or buffer.
                 * @function decode
                 * @memberof coretex.agent.v1.ContextHints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {coretex.agent.v1.ContextHints} ContextHints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ContextHints.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.coretex.agent.v1.ContextHints();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.maxInputTokens = reader.int32();
                                break;
                            }
                        case 2: {
                                message.allowSummarization = reader.bool();
                                break;
                            }
                        case 3: {
                                message.allowRetrieval = reader.bool();
                                break;
                            }
                        case 4: {
                                if (!(message.tags && message.tags.length))
                                    message.tags = [];
                                message.tags.push(reader.string());
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a ContextHints message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof coretex.agent.v1.ContextHints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {coretex.agent.v1.ContextHints} ContextHints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ContextHints.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a ContextHints message.
                 * @function verify
                 * @memberof coretex.agent.v1.ContextHints
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                ContextHints.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.maxInputTokens != null && message.hasOwnProperty("maxInputTokens"))
                        if (!$util.isInteger(message.maxInputTokens))
                            return "maxInputTokens: integer expected";
                    if (message.allowSummarization != null && message.hasOwnProperty("allowSummarization"))
                        if (typeof message.allowSummarization !== "boolean")
                            return "allowSummarization: boolean expected";
                    if (message.allowRetrieval != null && message.hasOwnProperty("allowRetrieval"))
                        if (typeof message.allowRetrieval !== "boolean")
                            return "allowRetrieval: boolean expected";
                    if (message.tags != null && message.hasOwnProperty("tags")) {
                        if (!Array.isArray(message.tags))
                            return "tags: array expected";
                        for (var i = 0; i < message.tags.length; ++i)
                            if (!$util.isString(message.tags[i]))
                                return "tags: string[] expected";
                    }
                    return null;
                };

                /**
                 * Creates a ContextHints message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof coretex.agent.v1.ContextHints
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {coretex.agent.v1.ContextHints} ContextHints
                 */
                ContextHints.fromObject = function fromObject(object) {
                    if (object instanceof $root.coretex.agent.v1.ContextHints)
                        return object;
                    var message = new $root.coretex.agent.v1.ContextHints();
                    if (object.maxInputTokens != null)
                        message.maxInputTokens = object.maxInputTokens | 0;
                    if (object.allowSummarization != null)
                        message.allowSummarization = Boolean(object.allowSummarization);
                    if (object.allowRetrieval != null)
                        message.allowRetrieval = Boolean(object.allowRetrieval);
                    if (object.tags) {
                        if (!Array.isArray(object.tags))
                            throw TypeError(".coretex.agent.v1.ContextHints.tags: array expected");
                        message.tags = [];
                        for (var i = 0; i < object.tags.length; ++i)
                            message.tags[i] = String(object.tags[i]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a ContextHints message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof coretex.agent.v1.ContextHints
                 * @static
                 * @param {coretex.agent.v1.ContextHints} message ContextHints
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                ContextHints.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults)
                        object.tags = [];
                    if (options.defaults) {
                        object.maxInputTokens = 0;
                        object.allowSummarization = false;
                        object.allowRetrieval = false;
                    }
                    if (message.maxInputTokens != null && message.hasOwnProperty("maxInputTokens"))
                        object.maxInputTokens = message.maxInputTokens;
                    if (message.allowSummarization != null && message.hasOwnProperty("allowSummarization"))
                        object.allowSummarization = message.allowSummarization;
                    if (message.allowRetrieval != null && message.hasOwnProperty("allowRetrieval"))
                        object.allowRetrieval = message.allowRetrieval;
                    if (message.tags && message.tags.length) {
                        object.tags = [];
                        for (var j = 0; j < message.tags.length; ++j)
                            object.tags[j] = message.tags[j];
                    }
                    return object;
                };

                /**
                 * Converts this ContextHints to JSON.
                 * @function toJSON
                 * @memberof coretex.agent.v1.ContextHints
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                ContextHints.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for ContextHints
                 * @function getTypeUrl
                 * @memberof coretex.agent.v1.ContextHints
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                ContextHints.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/coretex.agent.v1.ContextHints";
                };

                return ContextHints;
            })();

            v1.Budget = (function() {

                /**
                 * Properties of a Budget.
                 * @memberof coretex.agent.v1
                 * @interface IBudget
                 * @property {number|Long|null} [maxInputTokens] Budget maxInputTokens
                 * @property {number|Long|null} [maxOutputTokens] Budget maxOutputTokens
                 * @property {number|Long|null} [maxTotalTokens] Budget maxTotalTokens
                 * @property {number|Long|null} [deadlineMs] Budget deadlineMs
                 */

                /**
                 * Constructs a new Budget.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a Budget.
                 * @implements IBudget
                 * @constructor
                 * @param {coretex.agent.v1.IBudget=} [properties] Properties to set
                 */
                function Budget(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * Budget maxInputTokens.
                 * @member {number|Long} maxInputTokens
                 * @memberof coretex.agent.v1.Budget
                 * @instance
                 */
                Budget.prototype.maxInputTokens = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * Budget maxOutputTokens.
                 * @member {number|Long} maxOutputTokens
                 * @memberof coretex.agent.v1.Budget
                 * @instance
                 */
                Budget.prototype.maxOutputTokens = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * Budget maxTotalTokens.
                 * @member {number|Long} maxTotalTokens
                 * @memberof coretex.agent.v1.Budget
                 * @instance
                 */
                Budget.prototype.maxTotalTokens = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * Budget deadlineMs.
                 * @member {number|Long} deadlineMs
                 * @memberof coretex.agent.v1.Budget
                 * @instance
                 */
                Budget.prototype.deadlineMs = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * Creates a new Budget instance using the specified properties.
                 * @function create
                 * @memberof coretex.agent.v1.Budget
                 * @static
                 * @param {coretex.agent.v1.IBudget=} [properties] Properties to set
                 * @returns {coretex.agent.v1.Budget} Budget instance
                 */
                Budget.create = function create(properties) {
                    return new Budget(properties);
                };

                /**
                 * Encodes the specified Budget message. Does not implicitly {@link coretex.agent.v1.Budget.verify|verify} messages.
                 * @function encode
                 * @memberof coretex.agent.v1.Budget
                 * @static
                 * @param {coretex.agent.v1.IBudget} message Budget message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                Budget.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.maxInputTokens != null && Object.hasOwnProperty.call(message, "maxInputTokens"))
                        writer.uint32(/* id 1, wireType 0 =*/8).int64(message.maxInputTokens);
                    if (message.maxOutputTokens != null && Object.hasOwnProperty.call(message, "maxOutputTokens"))
                        writer.uint32(/* id 2, wireType 0 =*/16).int64(message.maxOutputTokens);
                    if (message.maxTotalTokens != null && Object.hasOwnProperty.call(message, "maxTotalTokens"))
                        writer.uint32(/* id 3, wireType 0 =*/24).int64(message.maxTotalTokens);
                    if (message.deadlineMs != null && Object.hasOwnProperty.call(message, "deadlineMs"))
                        writer.uint32(/* id 4, wireType 0 =*/32).int64(message.deadlineMs);
                    return writer;
                };

                /**
                 * Encodes the specified Budget message, length delimited. Does not implicitly {@link coretex.agent.v1.Budget.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof coretex.agent.v1.Budget
                 * @static
                 * @param {coretex.agent.v1.IBudget} message Budget message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                Budget.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a Budget message from the specified reader or buffer.
                 * @function decode
                 * @memberof coretex.agent.v1.Budget
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {coretex.agent.v1.Budget} Budget
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                Budget.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.coretex.agent.v1.Budget();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.maxInputTokens = reader.int64();
                                break;
                            }
                        case 2: {
                                message.maxOutputTokens = reader.int64();
                                break;
                            }
                        case 3: {
                                message.maxTotalTokens = reader.int64();
                                break;
                            }
                        case 4: {
                                message.deadlineMs = reader.int64();
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a Budget message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof coretex.agent.v1.Budget
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {coretex.agent.v1.Budget} Budget
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                Budget.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a Budget message.
                 * @function verify
                 * @memberof coretex.agent.v1.Budget
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                Budget.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.maxInputTokens != null && message.hasOwnProperty("maxInputTokens"))
                        if (!$util.isInteger(message.maxInputTokens) && !(message.maxInputTokens && $util.isInteger(message.maxInputTokens.low) && $util.isInteger(message.maxInputTokens.high)))
                            return "maxInputTokens: integer|Long expected";
                    if (message.maxOutputTokens != null && message.hasOwnProperty("maxOutputTokens"))
                        if (!$util.isInteger(message.maxOutputTokens) && !(message.maxOutputTokens && $util.isInteger(message.maxOutputTokens.low) && $util.isInteger(message.maxOutputTokens.high)))
                            return "maxOutputTokens: integer|Long expected";
                    if (message.maxTotalTokens != null && message.hasOwnProperty("maxTotalTokens"))
                        if (!$util.isInteger(message.maxTotalTokens) && !(message.maxTotalTokens && $util.isInteger(message.maxTotalTokens.low) && $util.isInteger(message.maxTotalTokens.high)))
                            return "maxTotalTokens: integer|Long expected";
                    if (message.deadlineMs != null && message.hasOwnProperty("deadlineMs"))
                        if (!$util.isInteger(message.deadlineMs) && !(message.deadlineMs && $util.isInteger(message.deadlineMs.low) && $util.isInteger(message.deadlineMs.high)))
                            return "deadlineMs: integer|Long expected";
                    return null;
                };

                /**
                 * Creates a Budget message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof coretex.agent.v1.Budget
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {coretex.agent.v1.Budget} Budget
                 */
                Budget.fromObject = function fromObject(object) {
                    if (object instanceof $root.coretex.agent.v1.Budget)
                        return object;
                    var message = new $root.coretex.agent.v1.Budget();
                    if (object.maxInputTokens != null)
                        if ($util.Long)
                            (message.maxInputTokens = $util.Long.fromValue(object.maxInputTokens)).unsigned = false;
                        else if (typeof object.maxInputTokens === "string")
                            message.maxInputTokens = parseInt(object.maxInputTokens, 10);
                        else if (typeof object.maxInputTokens === "number")
                            message.maxInputTokens = object.maxInputTokens;
                        else if (typeof object.maxInputTokens === "object")
                            message.maxInputTokens = new $util.LongBits(object.maxInputTokens.low >>> 0, object.maxInputTokens.high >>> 0).toNumber();
                    if (object.maxOutputTokens != null)
                        if ($util.Long)
                            (message.maxOutputTokens = $util.Long.fromValue(object.maxOutputTokens)).unsigned = false;
                        else if (typeof object.maxOutputTokens === "string")
                            message.maxOutputTokens = parseInt(object.maxOutputTokens, 10);
                        else if (typeof object.maxOutputTokens === "number")
                            message.maxOutputTokens = object.maxOutputTokens;
                        else if (typeof object.maxOutputTokens === "object")
                            message.maxOutputTokens = new $util.LongBits(object.maxOutputTokens.low >>> 0, object.maxOutputTokens.high >>> 0).toNumber();
                    if (object.maxTotalTokens != null)
                        if ($util.Long)
                            (message.maxTotalTokens = $util.Long.fromValue(object.maxTotalTokens)).unsigned = false;
                        else if (typeof object.maxTotalTokens === "string")
                            message.maxTotalTokens = parseInt(object.maxTotalTokens, 10);
                        else if (typeof object.maxTotalTokens === "number")
                            message.maxTotalTokens = object.maxTotalTokens;
                        else if (typeof object.maxTotalTokens === "object")
                            message.maxTotalTokens = new $util.LongBits(object.maxTotalTokens.low >>> 0, object.maxTotalTokens.high >>> 0).toNumber();
                    if (object.deadlineMs != null)
                        if ($util.Long)
                            (message.deadlineMs = $util.Long.fromValue(object.deadlineMs)).unsigned = false;
                        else if (typeof object.deadlineMs === "string")
                            message.deadlineMs = parseInt(object.deadlineMs, 10);
                        else if (typeof object.deadlineMs === "number")
                            message.deadlineMs = object.deadlineMs;
                        else if (typeof object.deadlineMs === "object")
                            message.deadlineMs = new $util.LongBits(object.deadlineMs.low >>> 0, object.deadlineMs.high >>> 0).toNumber();
                    return message;
                };

                /**
                 * Creates a plain object from a Budget message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof coretex.agent.v1.Budget
                 * @static
                 * @param {coretex.agent.v1.Budget} message Budget
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                Budget.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.defaults) {
                        if ($util.Long) {
                            var long = new $util.Long(0, 0, false);
                            object.maxInputTokens = options.longs === String ? long.toString() : options.longs === Number ? long.toNumber() : long;
                        } else
                            object.maxInputTokens = options.longs === String ? "0" : 0;
                        if ($util.Long) {
                            var long = new $util.Long(0, 0, false);
                            object.maxOutputTokens = options.longs === String ? long.toString() : options.longs === Number ? long.toNumber() : long;
                        } else
                            object.maxOutputTokens = options.longs === String ? "0" : 0;
                        if ($util.Long) {
                            var long = new $util.Long(0, 0, false);
                            object.maxTotalTokens = options.longs === String ? long.toString() : options.longs === Number ? long.toNumber() : long;
                        } else
                            object.maxTotalTokens = options.longs === String ? "0" : 0;
                        if ($util.Long) {
                            var long = new $util.Long(0, 0, false);
                            object.deadlineMs = options.longs === String ? long.toString() : options.longs === Number ? long.toNumber() : long;
                        } else
                            object.deadlineMs = options.longs === String ? "0" : 0;
                    }
                    if (message.maxInputTokens != null && message.hasOwnProperty("maxInputTokens"))
                        if (typeof message.maxInputTokens === "number")
                            object.maxInputTokens = options.longs === String ? String(message.maxInputTokens) : message.maxInputTokens;
                        else
                            object.maxInputTokens = options.longs === String ? $util.Long.prototype.toString.call(message.maxInputTokens) : options.longs === Number ? new $util.LongBits(message.maxInputTokens.low >>> 0, message.maxInputTokens.high >>> 0).toNumber() : message.maxInputTokens;
                    if (message.maxOutputTokens != null && message.hasOwnProperty("maxOutputTokens"))
                        if (typeof message.maxOutputTokens === "number")
                            object.maxOutputTokens = options.longs === String ? String(message.maxOutputTokens) : message.maxOutputTokens;
                        else
                            object.maxOutputTokens = options.longs === String ? $util.Long.prototype.toString.call(message.maxOutputTokens) : options.longs === Number ? new $util.LongBits(message.maxOutputTokens.low >>> 0, message.maxOutputTokens.high >>> 0).toNumber() : message.maxOutputTokens;
                    if (message.maxTotalTokens != null && message.hasOwnProperty("maxTotalTokens"))
                        if (typeof message.maxTotalTokens === "number")
                            object.maxTotalTokens = options.longs === String ? String(message.maxTotalTokens) : message.maxTotalTokens;
                        else
                            object.maxTotalTokens = options.longs === String ? $util.Long.prototype.toString.call(message.maxTotalTokens) : options.longs === Number ? new $util.LongBits(message.maxTotalTokens.low >>> 0, message.maxTotalTokens.high >>> 0).toNumber() : message.maxTotalTokens;
                    if (message.deadlineMs != null && message.hasOwnProperty("deadlineMs"))
                        if (typeof message.deadlineMs === "number")
                            object.deadlineMs = options.longs === String ? String(message.deadlineMs) : message.deadlineMs;
                        else
                            object.deadlineMs = options.longs === String ? $util.Long.prototype.toString.call(message.deadlineMs) : options.longs === Number ? new $util.LongBits(message.deadlineMs.low >>> 0, message.deadlineMs.high >>> 0).toNumber() : message.deadlineMs;
                    return object;
                };

                /**
                 * Converts this Budget to JSON.
                 * @function toJSON
                 * @memberof coretex.agent.v1.Budget
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                Budget.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for Budget
                 * @function getTypeUrl
                 * @memberof coretex.agent.v1.Budget
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                Budget.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/coretex.agent.v1.Budget";
                };

                return Budget;
            })();

            v1.JobRequest = (function() {

                /**
                 * Properties of a JobRequest.
                 * @memberof coretex.agent.v1
                 * @interface IJobRequest
                 * @property {string|null} [jobId] JobRequest jobId
                 * @property {string|null} [topic] JobRequest topic
                 * @property {coretex.agent.v1.JobPriority|null} [priority] JobRequest priority
                 * @property {string|null} [contextPtr] JobRequest contextPtr
                 * @property {string|null} [adapterId] JobRequest adapterId
                 * @property {Object.<string,string>|null} [env] JobRequest env
                 * @property {string|null} [parentJobId] JobRequest parentJobId
                 * @property {string|null} [workflowId] JobRequest workflowId
                 * @property {number|null} [stepIndex] JobRequest stepIndex
                 * @property {string|null} [memoryId] JobRequest memoryId
                 * @property {coretex.agent.v1.IContextHints|null} [contextHints] JobRequest contextHints
                 * @property {coretex.agent.v1.IBudget|null} [budget] JobRequest budget
                 * @property {string|null} [tenantId] JobRequest tenantId
                 * @property {string|null} [principalId] JobRequest principalId
                 * @property {Object.<string,string>|null} [labels] JobRequest labels
                 */

                /**
                 * Constructs a new JobRequest.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a JobRequest.
                 * @implements IJobRequest
                 * @constructor
                 * @param {coretex.agent.v1.IJobRequest=} [properties] Properties to set
                 */
                function JobRequest(properties) {
                    this.env = {};
                    this.labels = {};
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * JobRequest jobId.
                 * @member {string} jobId
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.jobId = "";

                /**
                 * JobRequest topic.
                 * @member {string} topic
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.topic = "";

                /**
                 * JobRequest priority.
                 * @member {coretex.agent.v1.JobPriority} priority
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.priority = 0;

                /**
                 * JobRequest contextPtr.
                 * @member {string} contextPtr
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.contextPtr = "";

                /**
                 * JobRequest adapterId.
                 * @member {string} adapterId
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.adapterId = "";

                /**
                 * JobRequest env.
                 * @member {Object.<string,string>} env
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.env = $util.emptyObject;

                /**
                 * JobRequest parentJobId.
                 * @member {string} parentJobId
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.parentJobId = "";

                /**
                 * JobRequest workflowId.
                 * @member {string} workflowId
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.workflowId = "";

                /**
                 * JobRequest stepIndex.
                 * @member {number} stepIndex
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.stepIndex = 0;

                /**
                 * JobRequest memoryId.
                 * @member {string} memoryId
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.memoryId = "";

                /**
                 * JobRequest contextHints.
                 * @member {coretex.agent.v1.IContextHints|null|undefined} contextHints
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.contextHints = null;

                /**
                 * JobRequest budget.
                 * @member {coretex.agent.v1.IBudget|null|undefined} budget
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.budget = null;

                /**
                 * JobRequest tenantId.
                 * @member {string} tenantId
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.tenantId = "";

                /**
                 * JobRequest principalId.
                 * @member {string} principalId
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.principalId = "";

                /**
                 * JobRequest labels.
                 * @member {Object.<string,string>} labels
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.labels = $util.emptyObject;

                /**
                 * Creates a new JobRequest instance using the specified properties.
                 * @function create
                 * @memberof coretex.agent.v1.JobRequest
                 * @static
                 * @param {coretex.agent.v1.IJobRequest=} [properties] Properties to set
                 * @returns {coretex.agent.v1.JobRequest} JobRequest instance
                 */
                JobRequest.create = function create(properties) {
                    return new JobRequest(properties);
                };

                /**
                 * Encodes the specified JobRequest message. Does not implicitly {@link coretex.agent.v1.JobRequest.verify|verify} messages.
                 * @function encode
                 * @memberof coretex.agent.v1.JobRequest
                 * @static
                 * @param {coretex.agent.v1.IJobRequest} message JobRequest message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobRequest.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.jobId != null && Object.hasOwnProperty.call(message, "jobId"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.jobId);
                    if (message.topic != null && Object.hasOwnProperty.call(message, "topic"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.topic);
                    if (message.priority != null && Object.hasOwnProperty.call(message, "priority"))
                        writer.uint32(/* id 3, wireType 0 =*/24).int32(message.priority);
                    if (message.contextPtr != null && Object.hasOwnProperty.call(message, "contextPtr"))
                        writer.uint32(/* id 4, wireType 2 =*/34).string(message.contextPtr);
                    if (message.adapterId != null && Object.hasOwnProperty.call(message, "adapterId"))
                        writer.uint32(/* id 5, wireType 2 =*/42).string(message.adapterId);
                    if (message.env != null && Object.hasOwnProperty.call(message, "env"))
                        for (var keys = Object.keys(message.env), i = 0; i < keys.length; ++i)
                            writer.uint32(/* id 6, wireType 2 =*/50).fork().uint32(/* id 1, wireType 2 =*/10).string(keys[i]).uint32(/* id 2, wireType 2 =*/18).string(message.env[keys[i]]).ldelim();
                    if (message.parentJobId != null && Object.hasOwnProperty.call(message, "parentJobId"))
                        writer.uint32(/* id 7, wireType 2 =*/58).string(message.parentJobId);
                    if (message.workflowId != null && Object.hasOwnProperty.call(message, "workflowId"))
                        writer.uint32(/* id 8, wireType 2 =*/66).string(message.workflowId);
                    if (message.stepIndex != null && Object.hasOwnProperty.call(message, "stepIndex"))
                        writer.uint32(/* id 9, wireType 0 =*/72).int32(message.stepIndex);
                    if (message.memoryId != null && Object.hasOwnProperty.call(message, "memoryId"))
                        writer.uint32(/* id 10, wireType 2 =*/82).string(message.memoryId);
                    if (message.contextHints != null && Object.hasOwnProperty.call(message, "contextHints"))
                        $root.coretex.agent.v1.ContextHints.encode(message.contextHints, writer.uint32(/* id 11, wireType 2 =*/90).fork()).ldelim();
                    if (message.budget != null && Object.hasOwnProperty.call(message, "budget"))
                        $root.coretex.agent.v1.Budget.encode(message.budget, writer.uint32(/* id 12, wireType 2 =*/98).fork()).ldelim();
                    if (message.tenantId != null && Object.hasOwnProperty.call(message, "tenantId"))
                        writer.uint32(/* id 13, wireType 2 =*/106).string(message.tenantId);
                    if (message.principalId != null && Object.hasOwnProperty.call(message, "principalId"))
                        writer.uint32(/* id 14, wireType 2 =*/114).string(message.principalId);
                    if (message.labels != null && Object.hasOwnProperty.call(message, "labels"))
                        for (var keys = Object.keys(message.labels), i = 0; i < keys.length; ++i)
                            writer.uint32(/* id 15, wireType 2 =*/122).fork().uint32(/* id 1, wireType 2 =*/10).string(keys[i]).uint32(/* id 2, wireType 2 =*/18).string(message.labels[keys[i]]).ldelim();
                    return writer;
                };

                /**
                 * Encodes the specified JobRequest message, length delimited. Does not implicitly {@link coretex.agent.v1.JobRequest.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof coretex.agent.v1.JobRequest
                 * @static
                 * @param {coretex.agent.v1.IJobRequest} message JobRequest message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobRequest.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a JobRequest message from the specified reader or buffer.
                 * @function decode
                 * @memberof coretex.agent.v1.JobRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {coretex.agent.v1.JobRequest} JobRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobRequest.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.coretex.agent.v1.JobRequest(), key, value;
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.jobId = reader.string();
                                break;
                            }
                        case 2: {
                                message.topic = reader.string();
                                break;
                            }
                        case 3: {
                                message.priority = reader.int32();
                                break;
                            }
                        case 4: {
                                message.contextPtr = reader.string();
                                break;
                            }
                        case 5: {
                                message.adapterId = reader.string();
                                break;
                            }
                        case 6: {
                                if (message.env === $util.emptyObject)
                                    message.env = {};
                                var end2 = reader.uint32() + reader.pos;
                                key = "";
                                value = "";
                                while (reader.pos < end2) {
                                    var tag2 = reader.uint32();
                                    switch (tag2 >>> 3) {
                                    case 1:
                                        key = reader.string();
                                        break;
                                    case 2:
                                        value = reader.string();
                                        break;
                                    default:
                                        reader.skipType(tag2 & 7);
                                        break;
                                    }
                                }
                                message.env[key] = value;
                                break;
                            }
                        case 7: {
                                message.parentJobId = reader.string();
                                break;
                            }
                        case 8: {
                                message.workflowId = reader.string();
                                break;
                            }
                        case 9: {
                                message.stepIndex = reader.int32();
                                break;
                            }
                        case 10: {
                                message.memoryId = reader.string();
                                break;
                            }
                        case 11: {
                                message.contextHints = $root.coretex.agent.v1.ContextHints.decode(reader, reader.uint32());
                                break;
                            }
                        case 12: {
                                message.budget = $root.coretex.agent.v1.Budget.decode(reader, reader.uint32());
                                break;
                            }
                        case 13: {
                                message.tenantId = reader.string();
                                break;
                            }
                        case 14: {
                                message.principalId = reader.string();
                                break;
                            }
                        case 15: {
                                if (message.labels === $util.emptyObject)
                                    message.labels = {};
                                var end2 = reader.uint32() + reader.pos;
                                key = "";
                                value = "";
                                while (reader.pos < end2) {
                                    var tag2 = reader.uint32();
                                    switch (tag2 >>> 3) {
                                    case 1:
                                        key = reader.string();
                                        break;
                                    case 2:
                                        value = reader.string();
                                        break;
                                    default:
                                        reader.skipType(tag2 & 7);
                                        break;
                                    }
                                }
                                message.labels[key] = value;
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a JobRequest message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof coretex.agent.v1.JobRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {coretex.agent.v1.JobRequest} JobRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobRequest.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a JobRequest message.
                 * @function verify
                 * @memberof coretex.agent.v1.JobRequest
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                JobRequest.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        if (!$util.isString(message.jobId))
                            return "jobId: string expected";
                    if (message.topic != null && message.hasOwnProperty("topic"))
                        if (!$util.isString(message.topic))
                            return "topic: string expected";
                    if (message.priority != null && message.hasOwnProperty("priority"))
                        switch (message.priority) {
                        default:
                            return "priority: enum value expected";
                        case 0:
                        case 1:
                        case 2:
                        case 3:
                            break;
                        }
                    if (message.contextPtr != null && message.hasOwnProperty("contextPtr"))
                        if (!$util.isString(message.contextPtr))
                            return "contextPtr: string expected";
                    if (message.adapterId != null && message.hasOwnProperty("adapterId"))
                        if (!$util.isString(message.adapterId))
                            return "adapterId: string expected";
                    if (message.env != null && message.hasOwnProperty("env")) {
                        if (!$util.isObject(message.env))
                            return "env: object expected";
                        var key = Object.keys(message.env);
                        for (var i = 0; i < key.length; ++i)
                            if (!$util.isString(message.env[key[i]]))
                                return "env: string{k:string} expected";
                    }
                    if (message.parentJobId != null && message.hasOwnProperty("parentJobId"))
                        if (!$util.isString(message.parentJobId))
                            return "parentJobId: string expected";
                    if (message.workflowId != null && message.hasOwnProperty("workflowId"))
                        if (!$util.isString(message.workflowId))
                            return "workflowId: string expected";
                    if (message.stepIndex != null && message.hasOwnProperty("stepIndex"))
                        if (!$util.isInteger(message.stepIndex))
                            return "stepIndex: integer expected";
                    if (message.memoryId != null && message.hasOwnProperty("memoryId"))
                        if (!$util.isString(message.memoryId))
                            return "memoryId: string expected";
                    if (message.contextHints != null && message.hasOwnProperty("contextHints")) {
                        var error = $root.coretex.agent.v1.ContextHints.verify(message.contextHints);
                        if (error)
                            return "contextHints." + error;
                    }
                    if (message.budget != null && message.hasOwnProperty("budget")) {
                        var error = $root.coretex.agent.v1.Budget.verify(message.budget);
                        if (error)
                            return "budget." + error;
                    }
                    if (message.tenantId != null && message.hasOwnProperty("tenantId"))
                        if (!$util.isString(message.tenantId))
                            return "tenantId: string expected";
                    if (message.principalId != null && message.hasOwnProperty("principalId"))
                        if (!$util.isString(message.principalId))
                            return "principalId: string expected";
                    if (message.labels != null && message.hasOwnProperty("labels")) {
                        if (!$util.isObject(message.labels))
                            return "labels: object expected";
                        var key = Object.keys(message.labels);
                        for (var i = 0; i < key.length; ++i)
                            if (!$util.isString(message.labels[key[i]]))
                                return "labels: string{k:string} expected";
                    }
                    return null;
                };

                /**
                 * Creates a JobRequest message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof coretex.agent.v1.JobRequest
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {coretex.agent.v1.JobRequest} JobRequest
                 */
                JobRequest.fromObject = function fromObject(object) {
                    if (object instanceof $root.coretex.agent.v1.JobRequest)
                        return object;
                    var message = new $root.coretex.agent.v1.JobRequest();
                    if (object.jobId != null)
                        message.jobId = String(object.jobId);
                    if (object.topic != null)
                        message.topic = String(object.topic);
                    switch (object.priority) {
                    default:
                        if (typeof object.priority === "number") {
                            message.priority = object.priority;
                            break;
                        }
                        break;
                    case "JOB_PRIORITY_UNSPECIFIED":
                    case 0:
                        message.priority = 0;
                        break;
                    case "JOB_PRIORITY_INTERACTIVE":
                    case 1:
                        message.priority = 1;
                        break;
                    case "JOB_PRIORITY_BATCH":
                    case 2:
                        message.priority = 2;
                        break;
                    case "JOB_PRIORITY_CRITICAL":
                    case 3:
                        message.priority = 3;
                        break;
                    }
                    if (object.contextPtr != null)
                        message.contextPtr = String(object.contextPtr);
                    if (object.adapterId != null)
                        message.adapterId = String(object.adapterId);
                    if (object.env) {
                        if (typeof object.env !== "object")
                            throw TypeError(".coretex.agent.v1.JobRequest.env: object expected");
                        message.env = {};
                        for (var keys = Object.keys(object.env), i = 0; i < keys.length; ++i)
                            message.env[keys[i]] = String(object.env[keys[i]]);
                    }
                    if (object.parentJobId != null)
                        message.parentJobId = String(object.parentJobId);
                    if (object.workflowId != null)
                        message.workflowId = String(object.workflowId);
                    if (object.stepIndex != null)
                        message.stepIndex = object.stepIndex | 0;
                    if (object.memoryId != null)
                        message.memoryId = String(object.memoryId);
                    if (object.contextHints != null) {
                        if (typeof object.contextHints !== "object")
                            throw TypeError(".coretex.agent.v1.JobRequest.contextHints: object expected");
                        message.contextHints = $root.coretex.agent.v1.ContextHints.fromObject(object.contextHints);
                    }
                    if (object.budget != null) {
                        if (typeof object.budget !== "object")
                            throw TypeError(".coretex.agent.v1.JobRequest.budget: object expected");
                        message.budget = $root.coretex.agent.v1.Budget.fromObject(object.budget);
                    }
                    if (object.tenantId != null)
                        message.tenantId = String(object.tenantId);
                    if (object.principalId != null)
                        message.principalId = String(object.principalId);
                    if (object.labels) {
                        if (typeof object.labels !== "object")
                            throw TypeError(".coretex.agent.v1.JobRequest.labels: object expected");
                        message.labels = {};
                        for (var keys = Object.keys(object.labels), i = 0; i < keys.length; ++i)
                            message.labels[keys[i]] = String(object.labels[keys[i]]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a JobRequest message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof coretex.agent.v1.JobRequest
                 * @static
                 * @param {coretex.agent.v1.JobRequest} message JobRequest
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                JobRequest.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.objects || options.defaults) {
                        object.env = {};
                        object.labels = {};
                    }
                    if (options.defaults) {
                        object.jobId = "";
                        object.topic = "";
                        object.priority = options.enums === String ? "JOB_PRIORITY_UNSPECIFIED" : 0;
                        object.contextPtr = "";
                        object.adapterId = "";
                        object.parentJobId = "";
                        object.workflowId = "";
                        object.stepIndex = 0;
                        object.memoryId = "";
                        object.contextHints = null;
                        object.budget = null;
                        object.tenantId = "";
                        object.principalId = "";
                    }
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        object.jobId = message.jobId;
                    if (message.topic != null && message.hasOwnProperty("topic"))
                        object.topic = message.topic;
                    if (message.priority != null && message.hasOwnProperty("priority"))
                        object.priority = options.enums === String ? $root.coretex.agent.v1.JobPriority[message.priority] === undefined ? message.priority : $root.coretex.agent.v1.JobPriority[message.priority] : message.priority;
                    if (message.contextPtr != null && message.hasOwnProperty("contextPtr"))
                        object.contextPtr = message.contextPtr;
                    if (message.adapterId != null && message.hasOwnProperty("adapterId"))
                        object.adapterId = message.adapterId;
                    var keys2;
                    if (message.env && (keys2 = Object.keys(message.env)).length) {
                        object.env = {};
                        for (var j = 0; j < keys2.length; ++j)
                            object.env[keys2[j]] = message.env[keys2[j]];
                    }
                    if (message.parentJobId != null && message.hasOwnProperty("parentJobId"))
                        object.parentJobId = message.parentJobId;
                    if (message.workflowId != null && message.hasOwnProperty("workflowId"))
                        object.workflowId = message.workflowId;
                    if (message.stepIndex != null && message.hasOwnProperty("stepIndex"))
                        object.stepIndex = message.stepIndex;
                    if (message.memoryId != null && message.hasOwnProperty("memoryId"))
                        object.memoryId = message.memoryId;
                    if (message.contextHints != null && message.hasOwnProperty("contextHints"))
                        object.contextHints = $root.coretex.agent.v1.ContextHints.toObject(message.contextHints, options);
                    if (message.budget != null && message.hasOwnProperty("budget"))
                        object.budget = $root.coretex.agent.v1.Budget.toObject(message.budget, options);
                    if (message.tenantId != null && message.hasOwnProperty("tenantId"))
                        object.tenantId = message.tenantId;
                    if (message.principalId != null && message.hasOwnProperty("principalId"))
                        object.principalId = message.principalId;
                    if (message.labels && (keys2 = Object.keys(message.labels)).length) {
                        object.labels = {};
                        for (var j = 0; j < keys2.length; ++j)
                            object.labels[keys2[j]] = message.labels[keys2[j]];
                    }
                    return object;
                };

                /**
                 * Converts this JobRequest to JSON.
                 * @function toJSON
                 * @memberof coretex.agent.v1.JobRequest
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                JobRequest.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for JobRequest
                 * @function getTypeUrl
                 * @memberof coretex.agent.v1.JobRequest
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                JobRequest.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/coretex.agent.v1.JobRequest";
                };

                return JobRequest;
            })();

            v1.JobResult = (function() {

                /**
                 * Properties of a JobResult.
                 * @memberof coretex.agent.v1
                 * @interface IJobResult
                 * @property {string|null} [jobId] JobResult jobId
                 * @property {coretex.agent.v1.JobStatus|null} [status] JobResult status
                 * @property {string|null} [resultPtr] JobResult resultPtr
                 * @property {string|null} [workerId] JobResult workerId
                 * @property {number|Long|null} [executionMs] JobResult executionMs
                 * @property {string|null} [errorCode] JobResult errorCode
                 * @property {string|null} [errorMessage] JobResult errorMessage
                 */

                /**
                 * Constructs a new JobResult.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a JobResult.
                 * @implements IJobResult
                 * @constructor
                 * @param {coretex.agent.v1.IJobResult=} [properties] Properties to set
                 */
                function JobResult(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * JobResult jobId.
                 * @member {string} jobId
                 * @memberof coretex.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.jobId = "";

                /**
                 * JobResult status.
                 * @member {coretex.agent.v1.JobStatus} status
                 * @memberof coretex.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.status = 0;

                /**
                 * JobResult resultPtr.
                 * @member {string} resultPtr
                 * @memberof coretex.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.resultPtr = "";

                /**
                 * JobResult workerId.
                 * @member {string} workerId
                 * @memberof coretex.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.workerId = "";

                /**
                 * JobResult executionMs.
                 * @member {number|Long} executionMs
                 * @memberof coretex.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.executionMs = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * JobResult errorCode.
                 * @member {string} errorCode
                 * @memberof coretex.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.errorCode = "";

                /**
                 * JobResult errorMessage.
                 * @member {string} errorMessage
                 * @memberof coretex.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.errorMessage = "";

                /**
                 * Creates a new JobResult instance using the specified properties.
                 * @function create
                 * @memberof coretex.agent.v1.JobResult
                 * @static
                 * @param {coretex.agent.v1.IJobResult=} [properties] Properties to set
                 * @returns {coretex.agent.v1.JobResult} JobResult instance
                 */
                JobResult.create = function create(properties) {
                    return new JobResult(properties);
                };

                /**
                 * Encodes the specified JobResult message. Does not implicitly {@link coretex.agent.v1.JobResult.verify|verify} messages.
                 * @function encode
                 * @memberof coretex.agent.v1.JobResult
                 * @static
                 * @param {coretex.agent.v1.IJobResult} message JobResult message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobResult.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.jobId != null && Object.hasOwnProperty.call(message, "jobId"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.jobId);
                    if (message.status != null && Object.hasOwnProperty.call(message, "status"))
                        writer.uint32(/* id 2, wireType 0 =*/16).int32(message.status);
                    if (message.resultPtr != null && Object.hasOwnProperty.call(message, "resultPtr"))
                        writer.uint32(/* id 3, wireType 2 =*/26).string(message.resultPtr);
                    if (message.workerId != null && Object.hasOwnProperty.call(message, "workerId"))
                        writer.uint32(/* id 4, wireType 2 =*/34).string(message.workerId);
                    if (message.executionMs != null && Object.hasOwnProperty.call(message, "executionMs"))
                        writer.uint32(/* id 5, wireType 0 =*/40).int64(message.executionMs);
                    if (message.errorCode != null && Object.hasOwnProperty.call(message, "errorCode"))
                        writer.uint32(/* id 6, wireType 2 =*/50).string(message.errorCode);
                    if (message.errorMessage != null && Object.hasOwnProperty.call(message, "errorMessage"))
                        writer.uint32(/* id 7, wireType 2 =*/58).string(message.errorMessage);
                    return writer;
                };

                /**
                 * Encodes the specified JobResult message, length delimited. Does not implicitly {@link coretex.agent.v1.JobResult.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof coretex.agent.v1.JobResult
                 * @static
                 * @param {coretex.agent.v1.IJobResult} message JobResult message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobResult.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a JobResult message from the specified reader or buffer.
                 * @function decode
                 * @memberof coretex.agent.v1.JobResult
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {coretex.agent.v1.JobResult} JobResult
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobResult.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.coretex.agent.v1.JobResult();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.jobId = reader.string();
                                break;
                            }
                        case 2: {
                                message.status = reader.int32();
                                break;
                            }
                        case 3: {
                                message.resultPtr = reader.string();
                                break;
                            }
                        case 4: {
                                message.workerId = reader.string();
                                break;
                            }
                        case 5: {
                                message.executionMs = reader.int64();
                                break;
                            }
                        case 6: {
                                message.errorCode = reader.string();
                                break;
                            }
                        case 7: {
                                message.errorMessage = reader.string();
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a JobResult message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof coretex.agent.v1.JobResult
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {coretex.agent.v1.JobResult} JobResult
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobResult.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a JobResult message.
                 * @function verify
                 * @memberof coretex.agent.v1.JobResult
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                JobResult.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        if (!$util.isString(message.jobId))
                            return "jobId: string expected";
                    if (message.status != null && message.hasOwnProperty("status"))
                        switch (message.status) {
                        default:
                            return "status: enum value expected";
                        case 0:
                        case 1:
                        case 2:
                        case 3:
                        case 4:
                        case 5:
                        case 6:
                        case 7:
                        case 8:
                        case 9:
                            break;
                        }
                    if (message.resultPtr != null && message.hasOwnProperty("resultPtr"))
                        if (!$util.isString(message.resultPtr))
                            return "resultPtr: string expected";
                    if (message.workerId != null && message.hasOwnProperty("workerId"))
                        if (!$util.isString(message.workerId))
                            return "workerId: string expected";
                    if (message.executionMs != null && message.hasOwnProperty("executionMs"))
                        if (!$util.isInteger(message.executionMs) && !(message.executionMs && $util.isInteger(message.executionMs.low) && $util.isInteger(message.executionMs.high)))
                            return "executionMs: integer|Long expected";
                    if (message.errorCode != null && message.hasOwnProperty("errorCode"))
                        if (!$util.isString(message.errorCode))
                            return "errorCode: string expected";
                    if (message.errorMessage != null && message.hasOwnProperty("errorMessage"))
                        if (!$util.isString(message.errorMessage))
                            return "errorMessage: string expected";
                    return null;
                };

                /**
                 * Creates a JobResult message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof coretex.agent.v1.JobResult
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {coretex.agent.v1.JobResult} JobResult
                 */
                JobResult.fromObject = function fromObject(object) {
                    if (object instanceof $root.coretex.agent.v1.JobResult)
                        return object;
                    var message = new $root.coretex.agent.v1.JobResult();
                    if (object.jobId != null)
                        message.jobId = String(object.jobId);
                    switch (object.status) {
                    default:
                        if (typeof object.status === "number") {
                            message.status = object.status;
                            break;
                        }
                        break;
                    case "JOB_STATUS_UNSPECIFIED":
                    case 0:
                        message.status = 0;
                        break;
                    case "JOB_STATUS_PENDING":
                    case 1:
                        message.status = 1;
                        break;
                    case "JOB_STATUS_SCHEDULED":
                    case 2:
                        message.status = 2;
                        break;
                    case "JOB_STATUS_DISPATCHED":
                    case 3:
                        message.status = 3;
                        break;
                    case "JOB_STATUS_RUNNING":
                    case 4:
                        message.status = 4;
                        break;
                    case "JOB_STATUS_SUCCEEDED":
                    case 5:
                        message.status = 5;
                        break;
                    case "JOB_STATUS_FAILED":
                    case 6:
                        message.status = 6;
                        break;
                    case "JOB_STATUS_CANCELLED":
                    case 7:
                        message.status = 7;
                        break;
                    case "JOB_STATUS_DENIED":
                    case 8:
                        message.status = 8;
                        break;
                    case "JOB_STATUS_TIMEOUT":
                    case 9:
                        message.status = 9;
                        break;
                    }
                    if (object.resultPtr != null)
                        message.resultPtr = String(object.resultPtr);
                    if (object.workerId != null)
                        message.workerId = String(object.workerId);
                    if (object.executionMs != null)
                        if ($util.Long)
                            (message.executionMs = $util.Long.fromValue(object.executionMs)).unsigned = false;
                        else if (typeof object.executionMs === "string")
                            message.executionMs = parseInt(object.executionMs, 10);
                        else if (typeof object.executionMs === "number")
                            message.executionMs = object.executionMs;
                        else if (typeof object.executionMs === "object")
                            message.executionMs = new $util.LongBits(object.executionMs.low >>> 0, object.executionMs.high >>> 0).toNumber();
                    if (object.errorCode != null)
                        message.errorCode = String(object.errorCode);
                    if (object.errorMessage != null)
                        message.errorMessage = String(object.errorMessage);
                    return message;
                };

                /**
                 * Creates a plain object from a JobResult message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof coretex.agent.v1.JobResult
                 * @static
                 * @param {coretex.agent.v1.JobResult} message JobResult
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                JobResult.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.defaults) {
                        object.jobId = "";
                        object.status = options.enums === String ? "JOB_STATUS_UNSPECIFIED" : 0;
                        object.resultPtr = "";
                        object.workerId = "";
                        if ($util.Long) {
                            var long = new $util.Long(0, 0, false);
                            object.executionMs = options.longs === String ? long.toString() : options.longs === Number ? long.toNumber() : long;
                        } else
                            object.executionMs = options.longs === String ? "0" : 0;
                        object.errorCode = "";
                        object.errorMessage = "";
                    }
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        object.jobId = message.jobId;
                    if (message.status != null && message.hasOwnProperty("status"))
                        object.status = options.enums === String ? $root.coretex.agent.v1.JobStatus[message.status] === undefined ? message.status : $root.coretex.agent.v1.JobStatus[message.status] : message.status;
                    if (message.resultPtr != null && message.hasOwnProperty("resultPtr"))
                        object.resultPtr = message.resultPtr;
                    if (message.workerId != null && message.hasOwnProperty("workerId"))
                        object.workerId = message.workerId;
                    if (message.executionMs != null && message.hasOwnProperty("executionMs"))
                        if (typeof message.executionMs === "number")
                            object.executionMs = options.longs === String ? String(message.executionMs) : message.executionMs;
                        else
                            object.executionMs = options.longs === String ? $util.Long.prototype.toString.call(message.executionMs) : options.longs === Number ? new $util.LongBits(message.executionMs.low >>> 0, message.executionMs.high >>> 0).toNumber() : message.executionMs;
                    if (message.errorCode != null && message.hasOwnProperty("errorCode"))
                        object.errorCode = message.errorCode;
                    if (message.errorMessage != null && message.hasOwnProperty("errorMessage"))
                        object.errorMessage = message.errorMessage;
                    return object;
                };

                /**
                 * Converts this JobResult to JSON.
                 * @function toJSON
                 * @memberof coretex.agent.v1.JobResult
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                JobResult.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for JobResult
                 * @function getTypeUrl
                 * @memberof coretex.agent.v1.JobResult
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                JobResult.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/coretex.agent.v1.JobResult";
                };

                return JobResult;
            })();

            v1.Heartbeat = (function() {

                /**
                 * Properties of a Heartbeat.
                 * @memberof coretex.agent.v1
                 * @interface IHeartbeat
                 * @property {string|null} [workerId] Heartbeat workerId
                 * @property {string|null} [region] Heartbeat region
                 * @property {string|null} [type] Heartbeat type
                 * @property {number|null} [cpuLoad] Heartbeat cpuLoad
                 * @property {number|null} [gpuUtilization] Heartbeat gpuUtilization
                 * @property {number|null} [activeJobs] Heartbeat activeJobs
                 * @property {Array.<string>|null} [capabilities] Heartbeat capabilities
                 * @property {string|null} [pool] Heartbeat pool
                 * @property {number|null} [maxParallelJobs] Heartbeat maxParallelJobs
                 */

                /**
                 * Constructs a new Heartbeat.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a Heartbeat.
                 * @implements IHeartbeat
                 * @constructor
                 * @param {coretex.agent.v1.IHeartbeat=} [properties] Properties to set
                 */
                function Heartbeat(properties) {
                    this.capabilities = [];
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * Heartbeat workerId.
                 * @member {string} workerId
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.workerId = "";

                /**
                 * Heartbeat region.
                 * @member {string} region
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.region = "";

                /**
                 * Heartbeat type.
                 * @member {string} type
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.type = "";

                /**
                 * Heartbeat cpuLoad.
                 * @member {number} cpuLoad
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.cpuLoad = 0;

                /**
                 * Heartbeat gpuUtilization.
                 * @member {number} gpuUtilization
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.gpuUtilization = 0;

                /**
                 * Heartbeat activeJobs.
                 * @member {number} activeJobs
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.activeJobs = 0;

                /**
                 * Heartbeat capabilities.
                 * @member {Array.<string>} capabilities
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.capabilities = $util.emptyArray;

                /**
                 * Heartbeat pool.
                 * @member {string} pool
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.pool = "";

                /**
                 * Heartbeat maxParallelJobs.
                 * @member {number} maxParallelJobs
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.maxParallelJobs = 0;

                /**
                 * Creates a new Heartbeat instance using the specified properties.
                 * @function create
                 * @memberof coretex.agent.v1.Heartbeat
                 * @static
                 * @param {coretex.agent.v1.IHeartbeat=} [properties] Properties to set
                 * @returns {coretex.agent.v1.Heartbeat} Heartbeat instance
                 */
                Heartbeat.create = function create(properties) {
                    return new Heartbeat(properties);
                };

                /**
                 * Encodes the specified Heartbeat message. Does not implicitly {@link coretex.agent.v1.Heartbeat.verify|verify} messages.
                 * @function encode
                 * @memberof coretex.agent.v1.Heartbeat
                 * @static
                 * @param {coretex.agent.v1.IHeartbeat} message Heartbeat message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                Heartbeat.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.workerId != null && Object.hasOwnProperty.call(message, "workerId"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.workerId);
                    if (message.region != null && Object.hasOwnProperty.call(message, "region"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.region);
                    if (message.type != null && Object.hasOwnProperty.call(message, "type"))
                        writer.uint32(/* id 3, wireType 2 =*/26).string(message.type);
                    if (message.cpuLoad != null && Object.hasOwnProperty.call(message, "cpuLoad"))
                        writer.uint32(/* id 4, wireType 5 =*/37).float(message.cpuLoad);
                    if (message.gpuUtilization != null && Object.hasOwnProperty.call(message, "gpuUtilization"))
                        writer.uint32(/* id 5, wireType 5 =*/45).float(message.gpuUtilization);
                    if (message.activeJobs != null && Object.hasOwnProperty.call(message, "activeJobs"))
                        writer.uint32(/* id 6, wireType 0 =*/48).int32(message.activeJobs);
                    if (message.capabilities != null && message.capabilities.length)
                        for (var i = 0; i < message.capabilities.length; ++i)
                            writer.uint32(/* id 7, wireType 2 =*/58).string(message.capabilities[i]);
                    if (message.pool != null && Object.hasOwnProperty.call(message, "pool"))
                        writer.uint32(/* id 11, wireType 2 =*/90).string(message.pool);
                    if (message.maxParallelJobs != null && Object.hasOwnProperty.call(message, "maxParallelJobs"))
                        writer.uint32(/* id 12, wireType 0 =*/96).int32(message.maxParallelJobs);
                    return writer;
                };

                /**
                 * Encodes the specified Heartbeat message, length delimited. Does not implicitly {@link coretex.agent.v1.Heartbeat.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof coretex.agent.v1.Heartbeat
                 * @static
                 * @param {coretex.agent.v1.IHeartbeat} message Heartbeat message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                Heartbeat.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a Heartbeat message from the specified reader or buffer.
                 * @function decode
                 * @memberof coretex.agent.v1.Heartbeat
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {coretex.agent.v1.Heartbeat} Heartbeat
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                Heartbeat.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.coretex.agent.v1.Heartbeat();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.workerId = reader.string();
                                break;
                            }
                        case 2: {
                                message.region = reader.string();
                                break;
                            }
                        case 3: {
                                message.type = reader.string();
                                break;
                            }
                        case 4: {
                                message.cpuLoad = reader.float();
                                break;
                            }
                        case 5: {
                                message.gpuUtilization = reader.float();
                                break;
                            }
                        case 6: {
                                message.activeJobs = reader.int32();
                                break;
                            }
                        case 7: {
                                if (!(message.capabilities && message.capabilities.length))
                                    message.capabilities = [];
                                message.capabilities.push(reader.string());
                                break;
                            }
                        case 11: {
                                message.pool = reader.string();
                                break;
                            }
                        case 12: {
                                message.maxParallelJobs = reader.int32();
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a Heartbeat message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof coretex.agent.v1.Heartbeat
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {coretex.agent.v1.Heartbeat} Heartbeat
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                Heartbeat.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a Heartbeat message.
                 * @function verify
                 * @memberof coretex.agent.v1.Heartbeat
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                Heartbeat.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.workerId != null && message.hasOwnProperty("workerId"))
                        if (!$util.isString(message.workerId))
                            return "workerId: string expected";
                    if (message.region != null && message.hasOwnProperty("region"))
                        if (!$util.isString(message.region))
                            return "region: string expected";
                    if (message.type != null && message.hasOwnProperty("type"))
                        if (!$util.isString(message.type))
                            return "type: string expected";
                    if (message.cpuLoad != null && message.hasOwnProperty("cpuLoad"))
                        if (typeof message.cpuLoad !== "number")
                            return "cpuLoad: number expected";
                    if (message.gpuUtilization != null && message.hasOwnProperty("gpuUtilization"))
                        if (typeof message.gpuUtilization !== "number")
                            return "gpuUtilization: number expected";
                    if (message.activeJobs != null && message.hasOwnProperty("activeJobs"))
                        if (!$util.isInteger(message.activeJobs))
                            return "activeJobs: integer expected";
                    if (message.capabilities != null && message.hasOwnProperty("capabilities")) {
                        if (!Array.isArray(message.capabilities))
                            return "capabilities: array expected";
                        for (var i = 0; i < message.capabilities.length; ++i)
                            if (!$util.isString(message.capabilities[i]))
                                return "capabilities: string[] expected";
                    }
                    if (message.pool != null && message.hasOwnProperty("pool"))
                        if (!$util.isString(message.pool))
                            return "pool: string expected";
                    if (message.maxParallelJobs != null && message.hasOwnProperty("maxParallelJobs"))
                        if (!$util.isInteger(message.maxParallelJobs))
                            return "maxParallelJobs: integer expected";
                    return null;
                };

                /**
                 * Creates a Heartbeat message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof coretex.agent.v1.Heartbeat
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {coretex.agent.v1.Heartbeat} Heartbeat
                 */
                Heartbeat.fromObject = function fromObject(object) {
                    if (object instanceof $root.coretex.agent.v1.Heartbeat)
                        return object;
                    var message = new $root.coretex.agent.v1.Heartbeat();
                    if (object.workerId != null)
                        message.workerId = String(object.workerId);
                    if (object.region != null)
                        message.region = String(object.region);
                    if (object.type != null)
                        message.type = String(object.type);
                    if (object.cpuLoad != null)
                        message.cpuLoad = Number(object.cpuLoad);
                    if (object.gpuUtilization != null)
                        message.gpuUtilization = Number(object.gpuUtilization);
                    if (object.activeJobs != null)
                        message.activeJobs = object.activeJobs | 0;
                    if (object.capabilities) {
                        if (!Array.isArray(object.capabilities))
                            throw TypeError(".coretex.agent.v1.Heartbeat.capabilities: array expected");
                        message.capabilities = [];
                        for (var i = 0; i < object.capabilities.length; ++i)
                            message.capabilities[i] = String(object.capabilities[i]);
                    }
                    if (object.pool != null)
                        message.pool = String(object.pool);
                    if (object.maxParallelJobs != null)
                        message.maxParallelJobs = object.maxParallelJobs | 0;
                    return message;
                };

                /**
                 * Creates a plain object from a Heartbeat message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof coretex.agent.v1.Heartbeat
                 * @static
                 * @param {coretex.agent.v1.Heartbeat} message Heartbeat
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                Heartbeat.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults)
                        object.capabilities = [];
                    if (options.defaults) {
                        object.workerId = "";
                        object.region = "";
                        object.type = "";
                        object.cpuLoad = 0;
                        object.gpuUtilization = 0;
                        object.activeJobs = 0;
                        object.pool = "";
                        object.maxParallelJobs = 0;
                    }
                    if (message.workerId != null && message.hasOwnProperty("workerId"))
                        object.workerId = message.workerId;
                    if (message.region != null && message.hasOwnProperty("region"))
                        object.region = message.region;
                    if (message.type != null && message.hasOwnProperty("type"))
                        object.type = message.type;
                    if (message.cpuLoad != null && message.hasOwnProperty("cpuLoad"))
                        object.cpuLoad = options.json && !isFinite(message.cpuLoad) ? String(message.cpuLoad) : message.cpuLoad;
                    if (message.gpuUtilization != null && message.hasOwnProperty("gpuUtilization"))
                        object.gpuUtilization = options.json && !isFinite(message.gpuUtilization) ? String(message.gpuUtilization) : message.gpuUtilization;
                    if (message.activeJobs != null && message.hasOwnProperty("activeJobs"))
                        object.activeJobs = message.activeJobs;
                    if (message.capabilities && message.capabilities.length) {
                        object.capabilities = [];
                        for (var j = 0; j < message.capabilities.length; ++j)
                            object.capabilities[j] = message.capabilities[j];
                    }
                    if (message.pool != null && message.hasOwnProperty("pool"))
                        object.pool = message.pool;
                    if (message.maxParallelJobs != null && message.hasOwnProperty("maxParallelJobs"))
                        object.maxParallelJobs = message.maxParallelJobs;
                    return object;
                };

                /**
                 * Converts this Heartbeat to JSON.
                 * @function toJSON
                 * @memberof coretex.agent.v1.Heartbeat
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                Heartbeat.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for Heartbeat
                 * @function getTypeUrl
                 * @memberof coretex.agent.v1.Heartbeat
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                Heartbeat.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/coretex.agent.v1.Heartbeat";
                };

                return Heartbeat;
            })();

            /**
             * DecisionType enum.
             * @name coretex.agent.v1.DecisionType
             * @enum {number}
             * @property {number} DECISION_TYPE_UNSPECIFIED=0 DECISION_TYPE_UNSPECIFIED value
             * @property {number} DECISION_TYPE_ALLOW=1 DECISION_TYPE_ALLOW value
             * @property {number} DECISION_TYPE_DENY=2 DECISION_TYPE_DENY value
             * @property {number} DECISION_TYPE_REQUIRE_HUMAN=3 DECISION_TYPE_REQUIRE_HUMAN value
             * @property {number} DECISION_TYPE_THROTTLE=4 DECISION_TYPE_THROTTLE value
             */
            v1.DecisionType = (function() {
                var valuesById = {}, values = Object.create(valuesById);
                values[valuesById[0] = "DECISION_TYPE_UNSPECIFIED"] = 0;
                values[valuesById[1] = "DECISION_TYPE_ALLOW"] = 1;
                values[valuesById[2] = "DECISION_TYPE_DENY"] = 2;
                values[valuesById[3] = "DECISION_TYPE_REQUIRE_HUMAN"] = 3;
                values[valuesById[4] = "DECISION_TYPE_THROTTLE"] = 4;
                return values;
            })();

            v1.PolicyCheckRequest = (function() {

                /**
                 * Properties of a PolicyCheckRequest.
                 * @memberof coretex.agent.v1
                 * @interface IPolicyCheckRequest
                 * @property {string|null} [jobId] PolicyCheckRequest jobId
                 * @property {string|null} [topic] PolicyCheckRequest topic
                 * @property {string|null} [tenant] PolicyCheckRequest tenant
                 * @property {coretex.agent.v1.JobPriority|null} [priority] PolicyCheckRequest priority
                 * @property {number|null} [estimatedCost] PolicyCheckRequest estimatedCost
                 * @property {coretex.agent.v1.IBudget|null} [budget] PolicyCheckRequest budget
                 * @property {string|null} [principalId] PolicyCheckRequest principalId
                 * @property {Object.<string,string>|null} [labels] PolicyCheckRequest labels
                 * @property {string|null} [memoryId] PolicyCheckRequest memoryId
                 */

                /**
                 * Constructs a new PolicyCheckRequest.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a PolicyCheckRequest.
                 * @implements IPolicyCheckRequest
                 * @constructor
                 * @param {coretex.agent.v1.IPolicyCheckRequest=} [properties] Properties to set
                 */
                function PolicyCheckRequest(properties) {
                    this.labels = {};
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * PolicyCheckRequest jobId.
                 * @member {string} jobId
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.jobId = "";

                /**
                 * PolicyCheckRequest topic.
                 * @member {string} topic
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.topic = "";

                /**
                 * PolicyCheckRequest tenant.
                 * @member {string} tenant
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.tenant = "";

                /**
                 * PolicyCheckRequest priority.
                 * @member {coretex.agent.v1.JobPriority} priority
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.priority = 0;

                /**
                 * PolicyCheckRequest estimatedCost.
                 * @member {number} estimatedCost
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.estimatedCost = 0;

                /**
                 * PolicyCheckRequest budget.
                 * @member {coretex.agent.v1.IBudget|null|undefined} budget
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.budget = null;

                /**
                 * PolicyCheckRequest principalId.
                 * @member {string} principalId
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.principalId = "";

                /**
                 * PolicyCheckRequest labels.
                 * @member {Object.<string,string>} labels
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.labels = $util.emptyObject;

                /**
                 * PolicyCheckRequest memoryId.
                 * @member {string} memoryId
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.memoryId = "";

                /**
                 * Creates a new PolicyCheckRequest instance using the specified properties.
                 * @function create
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {coretex.agent.v1.IPolicyCheckRequest=} [properties] Properties to set
                 * @returns {coretex.agent.v1.PolicyCheckRequest} PolicyCheckRequest instance
                 */
                PolicyCheckRequest.create = function create(properties) {
                    return new PolicyCheckRequest(properties);
                };

                /**
                 * Encodes the specified PolicyCheckRequest message. Does not implicitly {@link coretex.agent.v1.PolicyCheckRequest.verify|verify} messages.
                 * @function encode
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {coretex.agent.v1.IPolicyCheckRequest} message PolicyCheckRequest message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                PolicyCheckRequest.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.jobId != null && Object.hasOwnProperty.call(message, "jobId"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.jobId);
                    if (message.topic != null && Object.hasOwnProperty.call(message, "topic"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.topic);
                    if (message.tenant != null && Object.hasOwnProperty.call(message, "tenant"))
                        writer.uint32(/* id 3, wireType 2 =*/26).string(message.tenant);
                    if (message.priority != null && Object.hasOwnProperty.call(message, "priority"))
                        writer.uint32(/* id 4, wireType 0 =*/32).int32(message.priority);
                    if (message.estimatedCost != null && Object.hasOwnProperty.call(message, "estimatedCost"))
                        writer.uint32(/* id 5, wireType 1 =*/41).double(message.estimatedCost);
                    if (message.budget != null && Object.hasOwnProperty.call(message, "budget"))
                        $root.coretex.agent.v1.Budget.encode(message.budget, writer.uint32(/* id 6, wireType 2 =*/50).fork()).ldelim();
                    if (message.principalId != null && Object.hasOwnProperty.call(message, "principalId"))
                        writer.uint32(/* id 7, wireType 2 =*/58).string(message.principalId);
                    if (message.labels != null && Object.hasOwnProperty.call(message, "labels"))
                        for (var keys = Object.keys(message.labels), i = 0; i < keys.length; ++i)
                            writer.uint32(/* id 8, wireType 2 =*/66).fork().uint32(/* id 1, wireType 2 =*/10).string(keys[i]).uint32(/* id 2, wireType 2 =*/18).string(message.labels[keys[i]]).ldelim();
                    if (message.memoryId != null && Object.hasOwnProperty.call(message, "memoryId"))
                        writer.uint32(/* id 9, wireType 2 =*/74).string(message.memoryId);
                    return writer;
                };

                /**
                 * Encodes the specified PolicyCheckRequest message, length delimited. Does not implicitly {@link coretex.agent.v1.PolicyCheckRequest.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {coretex.agent.v1.IPolicyCheckRequest} message PolicyCheckRequest message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                PolicyCheckRequest.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a PolicyCheckRequest message from the specified reader or buffer.
                 * @function decode
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {coretex.agent.v1.PolicyCheckRequest} PolicyCheckRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                PolicyCheckRequest.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.coretex.agent.v1.PolicyCheckRequest(), key, value;
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.jobId = reader.string();
                                break;
                            }
                        case 2: {
                                message.topic = reader.string();
                                break;
                            }
                        case 3: {
                                message.tenant = reader.string();
                                break;
                            }
                        case 4: {
                                message.priority = reader.int32();
                                break;
                            }
                        case 5: {
                                message.estimatedCost = reader.double();
                                break;
                            }
                        case 6: {
                                message.budget = $root.coretex.agent.v1.Budget.decode(reader, reader.uint32());
                                break;
                            }
                        case 7: {
                                message.principalId = reader.string();
                                break;
                            }
                        case 8: {
                                if (message.labels === $util.emptyObject)
                                    message.labels = {};
                                var end2 = reader.uint32() + reader.pos;
                                key = "";
                                value = "";
                                while (reader.pos < end2) {
                                    var tag2 = reader.uint32();
                                    switch (tag2 >>> 3) {
                                    case 1:
                                        key = reader.string();
                                        break;
                                    case 2:
                                        value = reader.string();
                                        break;
                                    default:
                                        reader.skipType(tag2 & 7);
                                        break;
                                    }
                                }
                                message.labels[key] = value;
                                break;
                            }
                        case 9: {
                                message.memoryId = reader.string();
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a PolicyCheckRequest message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {coretex.agent.v1.PolicyCheckRequest} PolicyCheckRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                PolicyCheckRequest.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a PolicyCheckRequest message.
                 * @function verify
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                PolicyCheckRequest.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        if (!$util.isString(message.jobId))
                            return "jobId: string expected";
                    if (message.topic != null && message.hasOwnProperty("topic"))
                        if (!$util.isString(message.topic))
                            return "topic: string expected";
                    if (message.tenant != null && message.hasOwnProperty("tenant"))
                        if (!$util.isString(message.tenant))
                            return "tenant: string expected";
                    if (message.priority != null && message.hasOwnProperty("priority"))
                        switch (message.priority) {
                        default:
                            return "priority: enum value expected";
                        case 0:
                        case 1:
                        case 2:
                        case 3:
                            break;
                        }
                    if (message.estimatedCost != null && message.hasOwnProperty("estimatedCost"))
                        if (typeof message.estimatedCost !== "number")
                            return "estimatedCost: number expected";
                    if (message.budget != null && message.hasOwnProperty("budget")) {
                        var error = $root.coretex.agent.v1.Budget.verify(message.budget);
                        if (error)
                            return "budget." + error;
                    }
                    if (message.principalId != null && message.hasOwnProperty("principalId"))
                        if (!$util.isString(message.principalId))
                            return "principalId: string expected";
                    if (message.labels != null && message.hasOwnProperty("labels")) {
                        if (!$util.isObject(message.labels))
                            return "labels: object expected";
                        var key = Object.keys(message.labels);
                        for (var i = 0; i < key.length; ++i)
                            if (!$util.isString(message.labels[key[i]]))
                                return "labels: string{k:string} expected";
                    }
                    if (message.memoryId != null && message.hasOwnProperty("memoryId"))
                        if (!$util.isString(message.memoryId))
                            return "memoryId: string expected";
                    return null;
                };

                /**
                 * Creates a PolicyCheckRequest message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {coretex.agent.v1.PolicyCheckRequest} PolicyCheckRequest
                 */
                PolicyCheckRequest.fromObject = function fromObject(object) {
                    if (object instanceof $root.coretex.agent.v1.PolicyCheckRequest)
                        return object;
                    var message = new $root.coretex.agent.v1.PolicyCheckRequest();
                    if (object.jobId != null)
                        message.jobId = String(object.jobId);
                    if (object.topic != null)
                        message.topic = String(object.topic);
                    if (object.tenant != null)
                        message.tenant = String(object.tenant);
                    switch (object.priority) {
                    default:
                        if (typeof object.priority === "number") {
                            message.priority = object.priority;
                            break;
                        }
                        break;
                    case "JOB_PRIORITY_UNSPECIFIED":
                    case 0:
                        message.priority = 0;
                        break;
                    case "JOB_PRIORITY_INTERACTIVE":
                    case 1:
                        message.priority = 1;
                        break;
                    case "JOB_PRIORITY_BATCH":
                    case 2:
                        message.priority = 2;
                        break;
                    case "JOB_PRIORITY_CRITICAL":
                    case 3:
                        message.priority = 3;
                        break;
                    }
                    if (object.estimatedCost != null)
                        message.estimatedCost = Number(object.estimatedCost);
                    if (object.budget != null) {
                        if (typeof object.budget !== "object")
                            throw TypeError(".coretex.agent.v1.PolicyCheckRequest.budget: object expected");
                        message.budget = $root.coretex.agent.v1.Budget.fromObject(object.budget);
                    }
                    if (object.principalId != null)
                        message.principalId = String(object.principalId);
                    if (object.labels) {
                        if (typeof object.labels !== "object")
                            throw TypeError(".coretex.agent.v1.PolicyCheckRequest.labels: object expected");
                        message.labels = {};
                        for (var keys = Object.keys(object.labels), i = 0; i < keys.length; ++i)
                            message.labels[keys[i]] = String(object.labels[keys[i]]);
                    }
                    if (object.memoryId != null)
                        message.memoryId = String(object.memoryId);
                    return message;
                };

                /**
                 * Creates a plain object from a PolicyCheckRequest message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {coretex.agent.v1.PolicyCheckRequest} message PolicyCheckRequest
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                PolicyCheckRequest.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.objects || options.defaults)
                        object.labels = {};
                    if (options.defaults) {
                        object.jobId = "";
                        object.topic = "";
                        object.tenant = "";
                        object.priority = options.enums === String ? "JOB_PRIORITY_UNSPECIFIED" : 0;
                        object.estimatedCost = 0;
                        object.budget = null;
                        object.principalId = "";
                        object.memoryId = "";
                    }
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        object.jobId = message.jobId;
                    if (message.topic != null && message.hasOwnProperty("topic"))
                        object.topic = message.topic;
                    if (message.tenant != null && message.hasOwnProperty("tenant"))
                        object.tenant = message.tenant;
                    if (message.priority != null && message.hasOwnProperty("priority"))
                        object.priority = options.enums === String ? $root.coretex.agent.v1.JobPriority[message.priority] === undefined ? message.priority : $root.coretex.agent.v1.JobPriority[message.priority] : message.priority;
                    if (message.estimatedCost != null && message.hasOwnProperty("estimatedCost"))
                        object.estimatedCost = options.json && !isFinite(message.estimatedCost) ? String(message.estimatedCost) : message.estimatedCost;
                    if (message.budget != null && message.hasOwnProperty("budget"))
                        object.budget = $root.coretex.agent.v1.Budget.toObject(message.budget, options);
                    if (message.principalId != null && message.hasOwnProperty("principalId"))
                        object.principalId = message.principalId;
                    var keys2;
                    if (message.labels && (keys2 = Object.keys(message.labels)).length) {
                        object.labels = {};
                        for (var j = 0; j < keys2.length; ++j)
                            object.labels[keys2[j]] = message.labels[keys2[j]];
                    }
                    if (message.memoryId != null && message.hasOwnProperty("memoryId"))
                        object.memoryId = message.memoryId;
                    return object;
                };

                /**
                 * Converts this PolicyCheckRequest to JSON.
                 * @function toJSON
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                PolicyCheckRequest.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for PolicyCheckRequest
                 * @function getTypeUrl
                 * @memberof coretex.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                PolicyCheckRequest.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/coretex.agent.v1.PolicyCheckRequest";
                };

                return PolicyCheckRequest;
            })();

            v1.PolicyCheckResponse = (function() {

                /**
                 * Properties of a PolicyCheckResponse.
                 * @memberof coretex.agent.v1
                 * @interface IPolicyCheckResponse
                 * @property {coretex.agent.v1.DecisionType|null} [decision] PolicyCheckResponse decision
                 * @property {string|null} [reason] PolicyCheckResponse reason
                 * @property {string|null} [redactedContextPtr] PolicyCheckResponse redactedContextPtr
                 */

                /**
                 * Constructs a new PolicyCheckResponse.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a PolicyCheckResponse.
                 * @implements IPolicyCheckResponse
                 * @constructor
                 * @param {coretex.agent.v1.IPolicyCheckResponse=} [properties] Properties to set
                 */
                function PolicyCheckResponse(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * PolicyCheckResponse decision.
                 * @member {coretex.agent.v1.DecisionType} decision
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.decision = 0;

                /**
                 * PolicyCheckResponse reason.
                 * @member {string} reason
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.reason = "";

                /**
                 * PolicyCheckResponse redactedContextPtr.
                 * @member {string} redactedContextPtr
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.redactedContextPtr = "";

                /**
                 * Creates a new PolicyCheckResponse instance using the specified properties.
                 * @function create
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {coretex.agent.v1.IPolicyCheckResponse=} [properties] Properties to set
                 * @returns {coretex.agent.v1.PolicyCheckResponse} PolicyCheckResponse instance
                 */
                PolicyCheckResponse.create = function create(properties) {
                    return new PolicyCheckResponse(properties);
                };

                /**
                 * Encodes the specified PolicyCheckResponse message. Does not implicitly {@link coretex.agent.v1.PolicyCheckResponse.verify|verify} messages.
                 * @function encode
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {coretex.agent.v1.IPolicyCheckResponse} message PolicyCheckResponse message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                PolicyCheckResponse.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.decision != null && Object.hasOwnProperty.call(message, "decision"))
                        writer.uint32(/* id 1, wireType 0 =*/8).int32(message.decision);
                    if (message.reason != null && Object.hasOwnProperty.call(message, "reason"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.reason);
                    if (message.redactedContextPtr != null && Object.hasOwnProperty.call(message, "redactedContextPtr"))
                        writer.uint32(/* id 3, wireType 2 =*/26).string(message.redactedContextPtr);
                    return writer;
                };

                /**
                 * Encodes the specified PolicyCheckResponse message, length delimited. Does not implicitly {@link coretex.agent.v1.PolicyCheckResponse.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {coretex.agent.v1.IPolicyCheckResponse} message PolicyCheckResponse message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                PolicyCheckResponse.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a PolicyCheckResponse message from the specified reader or buffer.
                 * @function decode
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {coretex.agent.v1.PolicyCheckResponse} PolicyCheckResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                PolicyCheckResponse.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.coretex.agent.v1.PolicyCheckResponse();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.decision = reader.int32();
                                break;
                            }
                        case 2: {
                                message.reason = reader.string();
                                break;
                            }
                        case 3: {
                                message.redactedContextPtr = reader.string();
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a PolicyCheckResponse message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {coretex.agent.v1.PolicyCheckResponse} PolicyCheckResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                PolicyCheckResponse.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a PolicyCheckResponse message.
                 * @function verify
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                PolicyCheckResponse.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.decision != null && message.hasOwnProperty("decision"))
                        switch (message.decision) {
                        default:
                            return "decision: enum value expected";
                        case 0:
                        case 1:
                        case 2:
                        case 3:
                        case 4:
                            break;
                        }
                    if (message.reason != null && message.hasOwnProperty("reason"))
                        if (!$util.isString(message.reason))
                            return "reason: string expected";
                    if (message.redactedContextPtr != null && message.hasOwnProperty("redactedContextPtr"))
                        if (!$util.isString(message.redactedContextPtr))
                            return "redactedContextPtr: string expected";
                    return null;
                };

                /**
                 * Creates a PolicyCheckResponse message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {coretex.agent.v1.PolicyCheckResponse} PolicyCheckResponse
                 */
                PolicyCheckResponse.fromObject = function fromObject(object) {
                    if (object instanceof $root.coretex.agent.v1.PolicyCheckResponse)
                        return object;
                    var message = new $root.coretex.agent.v1.PolicyCheckResponse();
                    switch (object.decision) {
                    default:
                        if (typeof object.decision === "number") {
                            message.decision = object.decision;
                            break;
                        }
                        break;
                    case "DECISION_TYPE_UNSPECIFIED":
                    case 0:
                        message.decision = 0;
                        break;
                    case "DECISION_TYPE_ALLOW":
                    case 1:
                        message.decision = 1;
                        break;
                    case "DECISION_TYPE_DENY":
                    case 2:
                        message.decision = 2;
                        break;
                    case "DECISION_TYPE_REQUIRE_HUMAN":
                    case 3:
                        message.decision = 3;
                        break;
                    case "DECISION_TYPE_THROTTLE":
                    case 4:
                        message.decision = 4;
                        break;
                    }
                    if (object.reason != null)
                        message.reason = String(object.reason);
                    if (object.redactedContextPtr != null)
                        message.redactedContextPtr = String(object.redactedContextPtr);
                    return message;
                };

                /**
                 * Creates a plain object from a PolicyCheckResponse message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {coretex.agent.v1.PolicyCheckResponse} message PolicyCheckResponse
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                PolicyCheckResponse.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.defaults) {
                        object.decision = options.enums === String ? "DECISION_TYPE_UNSPECIFIED" : 0;
                        object.reason = "";
                        object.redactedContextPtr = "";
                    }
                    if (message.decision != null && message.hasOwnProperty("decision"))
                        object.decision = options.enums === String ? $root.coretex.agent.v1.DecisionType[message.decision] === undefined ? message.decision : $root.coretex.agent.v1.DecisionType[message.decision] : message.decision;
                    if (message.reason != null && message.hasOwnProperty("reason"))
                        object.reason = message.reason;
                    if (message.redactedContextPtr != null && message.hasOwnProperty("redactedContextPtr"))
                        object.redactedContextPtr = message.redactedContextPtr;
                    return object;
                };

                /**
                 * Converts this PolicyCheckResponse to JSON.
                 * @function toJSON
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                PolicyCheckResponse.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for PolicyCheckResponse
                 * @function getTypeUrl
                 * @memberof coretex.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                PolicyCheckResponse.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/coretex.agent.v1.PolicyCheckResponse";
                };

                return PolicyCheckResponse;
            })();

            v1.SafetyKernel = (function() {

                /**
                 * Constructs a new SafetyKernel service.
                 * @memberof coretex.agent.v1
                 * @classdesc Represents a SafetyKernel
                 * @extends $protobuf.rpc.Service
                 * @constructor
                 * @param {$protobuf.RPCImpl} rpcImpl RPC implementation
                 * @param {boolean} [requestDelimited=false] Whether requests are length-delimited
                 * @param {boolean} [responseDelimited=false] Whether responses are length-delimited
                 */
                function SafetyKernel(rpcImpl, requestDelimited, responseDelimited) {
                    $protobuf.rpc.Service.call(this, rpcImpl, requestDelimited, responseDelimited);
                }

                (SafetyKernel.prototype = Object.create($protobuf.rpc.Service.prototype)).constructor = SafetyKernel;

                /**
                 * Creates new SafetyKernel service using the specified rpc implementation.
                 * @function create
                 * @memberof coretex.agent.v1.SafetyKernel
                 * @static
                 * @param {$protobuf.RPCImpl} rpcImpl RPC implementation
                 * @param {boolean} [requestDelimited=false] Whether requests are length-delimited
                 * @param {boolean} [responseDelimited=false] Whether responses are length-delimited
                 * @returns {SafetyKernel} RPC service. Useful where requests and/or responses are streamed.
                 */
                SafetyKernel.create = function create(rpcImpl, requestDelimited, responseDelimited) {
                    return new this(rpcImpl, requestDelimited, responseDelimited);
                };

                /**
                 * Callback as used by {@link coretex.agent.v1.SafetyKernel#check}.
                 * @memberof coretex.agent.v1.SafetyKernel
                 * @typedef CheckCallback
                 * @type {function}
                 * @param {Error|null} error Error, if any
                 * @param {coretex.agent.v1.PolicyCheckResponse} [response] PolicyCheckResponse
                 */

                /**
                 * Calls Check.
                 * @function check
                 * @memberof coretex.agent.v1.SafetyKernel
                 * @instance
                 * @param {coretex.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @param {coretex.agent.v1.SafetyKernel.CheckCallback} callback Node-style callback called with the error, if any, and PolicyCheckResponse
                 * @returns {undefined}
                 * @variation 1
                 */
                Object.defineProperty(SafetyKernel.prototype.check = function check(request, callback) {
                    return this.rpcCall(check, $root.coretex.agent.v1.PolicyCheckRequest, $root.coretex.agent.v1.PolicyCheckResponse, request, callback);
                }, "name", { value: "Check" });

                /**
                 * Calls Check.
                 * @function check
                 * @memberof coretex.agent.v1.SafetyKernel
                 * @instance
                 * @param {coretex.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @returns {Promise<coretex.agent.v1.PolicyCheckResponse>} Promise
                 * @variation 2
                 */

                return SafetyKernel;
            })();

            return v1;
        })();

        return agent;
    })();

    return coretex;
})();

$root.google = (function() {

    /**
     * Namespace google.
     * @exports google
     * @namespace
     */
    var google = {};

    google.protobuf = (function() {

        /**
         * Namespace protobuf.
         * @memberof google
         * @namespace
         */
        var protobuf = {};

        protobuf.Timestamp = (function() {

            /**
             * Properties of a Timestamp.
             * @memberof google.protobuf
             * @interface ITimestamp
             * @property {number|Long|null} [seconds] Timestamp seconds
             * @property {number|null} [nanos] Timestamp nanos
             */

            /**
             * Constructs a new Timestamp.
             * @memberof google.protobuf
             * @classdesc Represents a Timestamp.
             * @implements ITimestamp
             * @constructor
             * @param {google.protobuf.ITimestamp=} [properties] Properties to set
             */
            function Timestamp(properties) {
                if (properties)
                    for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                        if (properties[keys[i]] != null)
                            this[keys[i]] = properties[keys[i]];
            }

            /**
             * Timestamp seconds.
             * @member {number|Long} seconds
             * @memberof google.protobuf.Timestamp
             * @instance
             */
            Timestamp.prototype.seconds = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

            /**
             * Timestamp nanos.
             * @member {number} nanos
             * @memberof google.protobuf.Timestamp
             * @instance
             */
            Timestamp.prototype.nanos = 0;

            /**
             * Creates a new Timestamp instance using the specified properties.
             * @function create
             * @memberof google.protobuf.Timestamp
             * @static
             * @param {google.protobuf.ITimestamp=} [properties] Properties to set
             * @returns {google.protobuf.Timestamp} Timestamp instance
             */
            Timestamp.create = function create(properties) {
                return new Timestamp(properties);
            };

            /**
             * Encodes the specified Timestamp message. Does not implicitly {@link google.protobuf.Timestamp.verify|verify} messages.
             * @function encode
             * @memberof google.protobuf.Timestamp
             * @static
             * @param {google.protobuf.ITimestamp} message Timestamp message or plain object to encode
             * @param {$protobuf.Writer} [writer] Writer to encode to
             * @returns {$protobuf.Writer} Writer
             */
            Timestamp.encode = function encode(message, writer) {
                if (!writer)
                    writer = $Writer.create();
                if (message.seconds != null && Object.hasOwnProperty.call(message, "seconds"))
                    writer.uint32(/* id 1, wireType 0 =*/8).int64(message.seconds);
                if (message.nanos != null && Object.hasOwnProperty.call(message, "nanos"))
                    writer.uint32(/* id 2, wireType 0 =*/16).int32(message.nanos);
                return writer;
            };

            /**
             * Encodes the specified Timestamp message, length delimited. Does not implicitly {@link google.protobuf.Timestamp.verify|verify} messages.
             * @function encodeDelimited
             * @memberof google.protobuf.Timestamp
             * @static
             * @param {google.protobuf.ITimestamp} message Timestamp message or plain object to encode
             * @param {$protobuf.Writer} [writer] Writer to encode to
             * @returns {$protobuf.Writer} Writer
             */
            Timestamp.encodeDelimited = function encodeDelimited(message, writer) {
                return this.encode(message, writer).ldelim();
            };

            /**
             * Decodes a Timestamp message from the specified reader or buffer.
             * @function decode
             * @memberof google.protobuf.Timestamp
             * @static
             * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
             * @param {number} [length] Message length if known beforehand
             * @returns {google.protobuf.Timestamp} Timestamp
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            Timestamp.decode = function decode(reader, length, error) {
                if (!(reader instanceof $Reader))
                    reader = $Reader.create(reader);
                var end = length === undefined ? reader.len : reader.pos + length, message = new $root.google.protobuf.Timestamp();
                while (reader.pos < end) {
                    var tag = reader.uint32();
                    if (tag === error)
                        break;
                    switch (tag >>> 3) {
                    case 1: {
                            message.seconds = reader.int64();
                            break;
                        }
                    case 2: {
                            message.nanos = reader.int32();
                            break;
                        }
                    default:
                        reader.skipType(tag & 7);
                        break;
                    }
                }
                return message;
            };

            /**
             * Decodes a Timestamp message from the specified reader or buffer, length delimited.
             * @function decodeDelimited
             * @memberof google.protobuf.Timestamp
             * @static
             * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
             * @returns {google.protobuf.Timestamp} Timestamp
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            Timestamp.decodeDelimited = function decodeDelimited(reader) {
                if (!(reader instanceof $Reader))
                    reader = new $Reader(reader);
                return this.decode(reader, reader.uint32());
            };

            /**
             * Verifies a Timestamp message.
             * @function verify
             * @memberof google.protobuf.Timestamp
             * @static
             * @param {Object.<string,*>} message Plain object to verify
             * @returns {string|null} `null` if valid, otherwise the reason why it is not
             */
            Timestamp.verify = function verify(message) {
                if (typeof message !== "object" || message === null)
                    return "object expected";
                if (message.seconds != null && message.hasOwnProperty("seconds"))
                    if (!$util.isInteger(message.seconds) && !(message.seconds && $util.isInteger(message.seconds.low) && $util.isInteger(message.seconds.high)))
                        return "seconds: integer|Long expected";
                if (message.nanos != null && message.hasOwnProperty("nanos"))
                    if (!$util.isInteger(message.nanos))
                        return "nanos: integer expected";
                return null;
            };

            /**
             * Creates a Timestamp message from a plain object. Also converts values to their respective internal types.
             * @function fromObject
             * @memberof google.protobuf.Timestamp
             * @static
             * @param {Object.<string,*>} object Plain object
             * @returns {google.protobuf.Timestamp} Timestamp
             */
            Timestamp.fromObject = function fromObject(object) {
                if (object instanceof $root.google.protobuf.Timestamp)
                    return object;
                var message = new $root.google.protobuf.Timestamp();
                if (object.seconds != null)
                    if ($util.Long)
                        (message.seconds = $util.Long.fromValue(object.seconds)).unsigned = false;
                    else if (typeof object.seconds === "string")
                        message.seconds = parseInt(object.seconds, 10);
                    else if (typeof object.seconds === "number")
                        message.seconds = object.seconds;
                    else if (typeof object.seconds === "object")
                        message.seconds = new $util.LongBits(object.seconds.low >>> 0, object.seconds.high >>> 0).toNumber();
                if (object.nanos != null)
                    message.nanos = object.nanos | 0;
                return message;
            };

            /**
             * Creates a plain object from a Timestamp message. Also converts values to other types if specified.
             * @function toObject
             * @memberof google.protobuf.Timestamp
             * @static
             * @param {google.protobuf.Timestamp} message Timestamp
             * @param {$protobuf.IConversionOptions} [options] Conversion options
             * @returns {Object.<string,*>} Plain object
             */
            Timestamp.toObject = function toObject(message, options) {
                if (!options)
                    options = {};
                var object = {};
                if (options.defaults) {
                    if ($util.Long) {
                        var long = new $util.Long(0, 0, false);
                        object.seconds = options.longs === String ? long.toString() : options.longs === Number ? long.toNumber() : long;
                    } else
                        object.seconds = options.longs === String ? "0" : 0;
                    object.nanos = 0;
                }
                if (message.seconds != null && message.hasOwnProperty("seconds"))
                    if (typeof message.seconds === "number")
                        object.seconds = options.longs === String ? String(message.seconds) : message.seconds;
                    else
                        object.seconds = options.longs === String ? $util.Long.prototype.toString.call(message.seconds) : options.longs === Number ? new $util.LongBits(message.seconds.low >>> 0, message.seconds.high >>> 0).toNumber() : message.seconds;
                if (message.nanos != null && message.hasOwnProperty("nanos"))
                    object.nanos = message.nanos;
                return object;
            };

            /**
             * Converts this Timestamp to JSON.
             * @function toJSON
             * @memberof google.protobuf.Timestamp
             * @instance
             * @returns {Object.<string,*>} JSON object
             */
            Timestamp.prototype.toJSON = function toJSON() {
                return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
            };

            /**
             * Gets the default type url for Timestamp
             * @function getTypeUrl
             * @memberof google.protobuf.Timestamp
             * @static
             * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
             * @returns {string} The default type url
             */
            Timestamp.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                if (typeUrlPrefix === undefined) {
                    typeUrlPrefix = "type.googleapis.com";
                }
                return typeUrlPrefix + "/google.protobuf.Timestamp";
            };

            return Timestamp;
        })();

        return protobuf;
    })();

    return google;
})();

module.exports = $root;
