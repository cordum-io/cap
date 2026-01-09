/*eslint-disable block-scoped-var, id-length, no-control-regex, no-magic-numbers, no-prototype-builtins, no-redeclare, no-shadow, no-var, sort-vars*/
"use strict";

var $protobuf = require("protobufjs/minimal");

// Common aliases
var $Reader = $protobuf.Reader, $Writer = $protobuf.Writer, $util = $protobuf.util;

// Exported root namespace
var $root = $protobuf.roots["cordum.agent.v1"] || ($protobuf.roots["cordum.agent.v1"] = {});

$root.cordum = (function() {

    /**
     * Namespace cordum.
     * @exports cordum
     * @namespace
     */
    var cordum = {};

    cordum.agent = (function() {

        /**
         * Namespace agent.
         * @memberof cordum
         * @namespace
         */
        var agent = {};

        agent.v1 = (function() {

            /**
             * Namespace v1.
             * @memberof cordum.agent
             * @namespace
             */
            var v1 = {};

            v1.SystemAlert = (function() {

                /**
                 * Properties of a SystemAlert.
                 * @memberof cordum.agent.v1
                 * @interface ISystemAlert
                 * @property {string|null} [level] SystemAlert level
                 * @property {string|null} [message] SystemAlert message
                 * @property {string|null} [component] SystemAlert component
                 * @property {string|null} [code] SystemAlert code
                 */

                /**
                 * Constructs a new SystemAlert.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a SystemAlert.
                 * @implements ISystemAlert
                 * @constructor
                 * @param {cordum.agent.v1.ISystemAlert=} [properties] Properties to set
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
                 * @memberof cordum.agent.v1.SystemAlert
                 * @instance
                 */
                SystemAlert.prototype.level = "";

                /**
                 * SystemAlert message.
                 * @member {string} message
                 * @memberof cordum.agent.v1.SystemAlert
                 * @instance
                 */
                SystemAlert.prototype.message = "";

                /**
                 * SystemAlert component.
                 * @member {string} component
                 * @memberof cordum.agent.v1.SystemAlert
                 * @instance
                 */
                SystemAlert.prototype.component = "";

                /**
                 * SystemAlert code.
                 * @member {string} code
                 * @memberof cordum.agent.v1.SystemAlert
                 * @instance
                 */
                SystemAlert.prototype.code = "";

                /**
                 * Creates a new SystemAlert instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.SystemAlert
                 * @static
                 * @param {cordum.agent.v1.ISystemAlert=} [properties] Properties to set
                 * @returns {cordum.agent.v1.SystemAlert} SystemAlert instance
                 */
                SystemAlert.create = function create(properties) {
                    return new SystemAlert(properties);
                };

                /**
                 * Encodes the specified SystemAlert message. Does not implicitly {@link cordum.agent.v1.SystemAlert.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.SystemAlert
                 * @static
                 * @param {cordum.agent.v1.ISystemAlert} message SystemAlert message or plain object to encode
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
                 * Encodes the specified SystemAlert message, length delimited. Does not implicitly {@link cordum.agent.v1.SystemAlert.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.SystemAlert
                 * @static
                 * @param {cordum.agent.v1.ISystemAlert} message SystemAlert message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                SystemAlert.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a SystemAlert message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.SystemAlert
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.SystemAlert} SystemAlert
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                SystemAlert.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.SystemAlert();
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
                 * @memberof cordum.agent.v1.SystemAlert
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.SystemAlert} SystemAlert
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
                 * @memberof cordum.agent.v1.SystemAlert
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
                 * @memberof cordum.agent.v1.SystemAlert
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.SystemAlert} SystemAlert
                 */
                SystemAlert.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.SystemAlert)
                        return object;
                    var message = new $root.cordum.agent.v1.SystemAlert();
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
                 * @memberof cordum.agent.v1.SystemAlert
                 * @static
                 * @param {cordum.agent.v1.SystemAlert} message SystemAlert
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
                 * @memberof cordum.agent.v1.SystemAlert
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                SystemAlert.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for SystemAlert
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.SystemAlert
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                SystemAlert.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.SystemAlert";
                };

                return SystemAlert;
            })();

            v1.BusPacket = (function() {

                /**
                 * Properties of a BusPacket.
                 * @memberof cordum.agent.v1
                 * @interface IBusPacket
                 * @property {string|null} [traceId] BusPacket traceId
                 * @property {string|null} [senderId] BusPacket senderId
                 * @property {google.protobuf.ITimestamp|null} [createdAt] BusPacket createdAt
                 * @property {number|null} [protocolVersion] BusPacket protocolVersion
                 * @property {cordum.agent.v1.IJobRequest|null} [jobRequest] BusPacket jobRequest
                 * @property {cordum.agent.v1.IJobResult|null} [jobResult] BusPacket jobResult
                 * @property {cordum.agent.v1.IHeartbeat|null} [heartbeat] BusPacket heartbeat
                 * @property {cordum.agent.v1.ISystemAlert|null} [alert] BusPacket alert
                 * @property {cordum.agent.v1.IJobProgress|null} [jobProgress] BusPacket jobProgress
                 * @property {cordum.agent.v1.IJobCancel|null} [jobCancel] BusPacket jobCancel
                 * @property {Uint8Array|null} [signature] BusPacket signature
                 */

                /**
                 * Constructs a new BusPacket.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a BusPacket.
                 * @implements IBusPacket
                 * @constructor
                 * @param {cordum.agent.v1.IBusPacket=} [properties] Properties to set
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
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.traceId = "";

                /**
                 * BusPacket senderId.
                 * @member {string} senderId
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.senderId = "";

                /**
                 * BusPacket createdAt.
                 * @member {google.protobuf.ITimestamp|null|undefined} createdAt
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.createdAt = null;

                /**
                 * BusPacket protocolVersion.
                 * @member {number} protocolVersion
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.protocolVersion = 0;

                /**
                 * BusPacket jobRequest.
                 * @member {cordum.agent.v1.IJobRequest|null|undefined} jobRequest
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.jobRequest = null;

                /**
                 * BusPacket jobResult.
                 * @member {cordum.agent.v1.IJobResult|null|undefined} jobResult
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.jobResult = null;

                /**
                 * BusPacket heartbeat.
                 * @member {cordum.agent.v1.IHeartbeat|null|undefined} heartbeat
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.heartbeat = null;

                /**
                 * BusPacket alert.
                 * @member {cordum.agent.v1.ISystemAlert|null|undefined} alert
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.alert = null;

                /**
                 * BusPacket jobProgress.
                 * @member {cordum.agent.v1.IJobProgress|null|undefined} jobProgress
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.jobProgress = null;

                /**
                 * BusPacket jobCancel.
                 * @member {cordum.agent.v1.IJobCancel|null|undefined} jobCancel
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.jobCancel = null;

                /**
                 * BusPacket signature.
                 * @member {Uint8Array} signature
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                BusPacket.prototype.signature = $util.newBuffer([]);

                // OneOf field names bound to virtual getters and setters
                var $oneOfFields;

                /**
                 * BusPacket payload.
                 * @member {"jobRequest"|"jobResult"|"heartbeat"|"alert"|"jobProgress"|"jobCancel"|undefined} payload
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 */
                Object.defineProperty(BusPacket.prototype, "payload", {
                    get: $util.oneOfGetter($oneOfFields = ["jobRequest", "jobResult", "heartbeat", "alert", "jobProgress", "jobCancel"]),
                    set: $util.oneOfSetter($oneOfFields)
                });

                /**
                 * Creates a new BusPacket instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.BusPacket
                 * @static
                 * @param {cordum.agent.v1.IBusPacket=} [properties] Properties to set
                 * @returns {cordum.agent.v1.BusPacket} BusPacket instance
                 */
                BusPacket.create = function create(properties) {
                    return new BusPacket(properties);
                };

                /**
                 * Encodes the specified BusPacket message. Does not implicitly {@link cordum.agent.v1.BusPacket.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.BusPacket
                 * @static
                 * @param {cordum.agent.v1.IBusPacket} message BusPacket message or plain object to encode
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
                        $root.cordum.agent.v1.JobRequest.encode(message.jobRequest, writer.uint32(/* id 10, wireType 2 =*/82).fork()).ldelim();
                    if (message.jobResult != null && Object.hasOwnProperty.call(message, "jobResult"))
                        $root.cordum.agent.v1.JobResult.encode(message.jobResult, writer.uint32(/* id 11, wireType 2 =*/90).fork()).ldelim();
                    if (message.heartbeat != null && Object.hasOwnProperty.call(message, "heartbeat"))
                        $root.cordum.agent.v1.Heartbeat.encode(message.heartbeat, writer.uint32(/* id 12, wireType 2 =*/98).fork()).ldelim();
                    if (message.alert != null && Object.hasOwnProperty.call(message, "alert"))
                        $root.cordum.agent.v1.SystemAlert.encode(message.alert, writer.uint32(/* id 13, wireType 2 =*/106).fork()).ldelim();
                    if (message.signature != null && Object.hasOwnProperty.call(message, "signature"))
                        writer.uint32(/* id 14, wireType 2 =*/114).bytes(message.signature);
                    if (message.jobProgress != null && Object.hasOwnProperty.call(message, "jobProgress"))
                        $root.cordum.agent.v1.JobProgress.encode(message.jobProgress, writer.uint32(/* id 15, wireType 2 =*/122).fork()).ldelim();
                    if (message.jobCancel != null && Object.hasOwnProperty.call(message, "jobCancel"))
                        $root.cordum.agent.v1.JobCancel.encode(message.jobCancel, writer.uint32(/* id 16, wireType 2 =*/130).fork()).ldelim();
                    return writer;
                };

                /**
                 * Encodes the specified BusPacket message, length delimited. Does not implicitly {@link cordum.agent.v1.BusPacket.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.BusPacket
                 * @static
                 * @param {cordum.agent.v1.IBusPacket} message BusPacket message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                BusPacket.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a BusPacket message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.BusPacket
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.BusPacket} BusPacket
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                BusPacket.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.BusPacket();
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
                                message.jobRequest = $root.cordum.agent.v1.JobRequest.decode(reader, reader.uint32());
                                break;
                            }
                        case 11: {
                                message.jobResult = $root.cordum.agent.v1.JobResult.decode(reader, reader.uint32());
                                break;
                            }
                        case 12: {
                                message.heartbeat = $root.cordum.agent.v1.Heartbeat.decode(reader, reader.uint32());
                                break;
                            }
                        case 13: {
                                message.alert = $root.cordum.agent.v1.SystemAlert.decode(reader, reader.uint32());
                                break;
                            }
                        case 15: {
                                message.jobProgress = $root.cordum.agent.v1.JobProgress.decode(reader, reader.uint32());
                                break;
                            }
                        case 16: {
                                message.jobCancel = $root.cordum.agent.v1.JobCancel.decode(reader, reader.uint32());
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
                 * @memberof cordum.agent.v1.BusPacket
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.BusPacket} BusPacket
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
                 * @memberof cordum.agent.v1.BusPacket
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
                            var error = $root.cordum.agent.v1.JobRequest.verify(message.jobRequest);
                            if (error)
                                return "jobRequest." + error;
                        }
                    }
                    if (message.jobResult != null && message.hasOwnProperty("jobResult")) {
                        if (properties.payload === 1)
                            return "payload: multiple values";
                        properties.payload = 1;
                        {
                            var error = $root.cordum.agent.v1.JobResult.verify(message.jobResult);
                            if (error)
                                return "jobResult." + error;
                        }
                    }
                    if (message.heartbeat != null && message.hasOwnProperty("heartbeat")) {
                        if (properties.payload === 1)
                            return "payload: multiple values";
                        properties.payload = 1;
                        {
                            var error = $root.cordum.agent.v1.Heartbeat.verify(message.heartbeat);
                            if (error)
                                return "heartbeat." + error;
                        }
                    }
                    if (message.alert != null && message.hasOwnProperty("alert")) {
                        if (properties.payload === 1)
                            return "payload: multiple values";
                        properties.payload = 1;
                        {
                            var error = $root.cordum.agent.v1.SystemAlert.verify(message.alert);
                            if (error)
                                return "alert." + error;
                        }
                    }
                    if (message.jobProgress != null && message.hasOwnProperty("jobProgress")) {
                        if (properties.payload === 1)
                            return "payload: multiple values";
                        properties.payload = 1;
                        {
                            var error = $root.cordum.agent.v1.JobProgress.verify(message.jobProgress);
                            if (error)
                                return "jobProgress." + error;
                        }
                    }
                    if (message.jobCancel != null && message.hasOwnProperty("jobCancel")) {
                        if (properties.payload === 1)
                            return "payload: multiple values";
                        properties.payload = 1;
                        {
                            var error = $root.cordum.agent.v1.JobCancel.verify(message.jobCancel);
                            if (error)
                                return "jobCancel." + error;
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
                 * @memberof cordum.agent.v1.BusPacket
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.BusPacket} BusPacket
                 */
                BusPacket.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.BusPacket)
                        return object;
                    var message = new $root.cordum.agent.v1.BusPacket();
                    if (object.traceId != null)
                        message.traceId = String(object.traceId);
                    if (object.senderId != null)
                        message.senderId = String(object.senderId);
                    if (object.createdAt != null) {
                        if (typeof object.createdAt !== "object")
                            throw TypeError(".cordum.agent.v1.BusPacket.createdAt: object expected");
                        message.createdAt = $root.google.protobuf.Timestamp.fromObject(object.createdAt);
                    }
                    if (object.protocolVersion != null)
                        message.protocolVersion = object.protocolVersion | 0;
                    if (object.jobRequest != null) {
                        if (typeof object.jobRequest !== "object")
                            throw TypeError(".cordum.agent.v1.BusPacket.jobRequest: object expected");
                        message.jobRequest = $root.cordum.agent.v1.JobRequest.fromObject(object.jobRequest);
                    }
                    if (object.jobResult != null) {
                        if (typeof object.jobResult !== "object")
                            throw TypeError(".cordum.agent.v1.BusPacket.jobResult: object expected");
                        message.jobResult = $root.cordum.agent.v1.JobResult.fromObject(object.jobResult);
                    }
                    if (object.heartbeat != null) {
                        if (typeof object.heartbeat !== "object")
                            throw TypeError(".cordum.agent.v1.BusPacket.heartbeat: object expected");
                        message.heartbeat = $root.cordum.agent.v1.Heartbeat.fromObject(object.heartbeat);
                    }
                    if (object.alert != null) {
                        if (typeof object.alert !== "object")
                            throw TypeError(".cordum.agent.v1.BusPacket.alert: object expected");
                        message.alert = $root.cordum.agent.v1.SystemAlert.fromObject(object.alert);
                    }
                    if (object.jobProgress != null) {
                        if (typeof object.jobProgress !== "object")
                            throw TypeError(".cordum.agent.v1.BusPacket.jobProgress: object expected");
                        message.jobProgress = $root.cordum.agent.v1.JobProgress.fromObject(object.jobProgress);
                    }
                    if (object.jobCancel != null) {
                        if (typeof object.jobCancel !== "object")
                            throw TypeError(".cordum.agent.v1.BusPacket.jobCancel: object expected");
                        message.jobCancel = $root.cordum.agent.v1.JobCancel.fromObject(object.jobCancel);
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
                 * @memberof cordum.agent.v1.BusPacket
                 * @static
                 * @param {cordum.agent.v1.BusPacket} message BusPacket
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
                        object.jobRequest = $root.cordum.agent.v1.JobRequest.toObject(message.jobRequest, options);
                        if (options.oneofs)
                            object.payload = "jobRequest";
                    }
                    if (message.jobResult != null && message.hasOwnProperty("jobResult")) {
                        object.jobResult = $root.cordum.agent.v1.JobResult.toObject(message.jobResult, options);
                        if (options.oneofs)
                            object.payload = "jobResult";
                    }
                    if (message.heartbeat != null && message.hasOwnProperty("heartbeat")) {
                        object.heartbeat = $root.cordum.agent.v1.Heartbeat.toObject(message.heartbeat, options);
                        if (options.oneofs)
                            object.payload = "heartbeat";
                    }
                    if (message.alert != null && message.hasOwnProperty("alert")) {
                        object.alert = $root.cordum.agent.v1.SystemAlert.toObject(message.alert, options);
                        if (options.oneofs)
                            object.payload = "alert";
                    }
                    if (message.signature != null && message.hasOwnProperty("signature"))
                        object.signature = options.bytes === String ? $util.base64.encode(message.signature, 0, message.signature.length) : options.bytes === Array ? Array.prototype.slice.call(message.signature) : message.signature;
                    if (message.jobProgress != null && message.hasOwnProperty("jobProgress")) {
                        object.jobProgress = $root.cordum.agent.v1.JobProgress.toObject(message.jobProgress, options);
                        if (options.oneofs)
                            object.payload = "jobProgress";
                    }
                    if (message.jobCancel != null && message.hasOwnProperty("jobCancel")) {
                        object.jobCancel = $root.cordum.agent.v1.JobCancel.toObject(message.jobCancel, options);
                        if (options.oneofs)
                            object.payload = "jobCancel";
                    }
                    return object;
                };

                /**
                 * Converts this BusPacket to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.BusPacket
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                BusPacket.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for BusPacket
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.BusPacket
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                BusPacket.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.BusPacket";
                };

                return BusPacket;
            })();

            /**
             * JobPriority enum.
             * @name cordum.agent.v1.JobPriority
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
             * @name cordum.agent.v1.JobStatus
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
                 * @memberof cordum.agent.v1
                 * @interface IContextHints
                 * @property {number|null} [maxInputTokens] ContextHints maxInputTokens
                 * @property {boolean|null} [allowSummarization] ContextHints allowSummarization
                 * @property {boolean|null} [allowRetrieval] ContextHints allowRetrieval
                 * @property {Array.<string>|null} [tags] ContextHints tags
                 */

                /**
                 * Constructs a new ContextHints.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a ContextHints.
                 * @implements IContextHints
                 * @constructor
                 * @param {cordum.agent.v1.IContextHints=} [properties] Properties to set
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
                 * @memberof cordum.agent.v1.ContextHints
                 * @instance
                 */
                ContextHints.prototype.maxInputTokens = 0;

                /**
                 * ContextHints allowSummarization.
                 * @member {boolean} allowSummarization
                 * @memberof cordum.agent.v1.ContextHints
                 * @instance
                 */
                ContextHints.prototype.allowSummarization = false;

                /**
                 * ContextHints allowRetrieval.
                 * @member {boolean} allowRetrieval
                 * @memberof cordum.agent.v1.ContextHints
                 * @instance
                 */
                ContextHints.prototype.allowRetrieval = false;

                /**
                 * ContextHints tags.
                 * @member {Array.<string>} tags
                 * @memberof cordum.agent.v1.ContextHints
                 * @instance
                 */
                ContextHints.prototype.tags = $util.emptyArray;

                /**
                 * Creates a new ContextHints instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.ContextHints
                 * @static
                 * @param {cordum.agent.v1.IContextHints=} [properties] Properties to set
                 * @returns {cordum.agent.v1.ContextHints} ContextHints instance
                 */
                ContextHints.create = function create(properties) {
                    return new ContextHints(properties);
                };

                /**
                 * Encodes the specified ContextHints message. Does not implicitly {@link cordum.agent.v1.ContextHints.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.ContextHints
                 * @static
                 * @param {cordum.agent.v1.IContextHints} message ContextHints message or plain object to encode
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
                 * Encodes the specified ContextHints message, length delimited. Does not implicitly {@link cordum.agent.v1.ContextHints.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.ContextHints
                 * @static
                 * @param {cordum.agent.v1.IContextHints} message ContextHints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ContextHints.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a ContextHints message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.ContextHints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.ContextHints} ContextHints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ContextHints.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.ContextHints();
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
                 * @memberof cordum.agent.v1.ContextHints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.ContextHints} ContextHints
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
                 * @memberof cordum.agent.v1.ContextHints
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
                 * @memberof cordum.agent.v1.ContextHints
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.ContextHints} ContextHints
                 */
                ContextHints.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.ContextHints)
                        return object;
                    var message = new $root.cordum.agent.v1.ContextHints();
                    if (object.maxInputTokens != null)
                        message.maxInputTokens = object.maxInputTokens | 0;
                    if (object.allowSummarization != null)
                        message.allowSummarization = Boolean(object.allowSummarization);
                    if (object.allowRetrieval != null)
                        message.allowRetrieval = Boolean(object.allowRetrieval);
                    if (object.tags) {
                        if (!Array.isArray(object.tags))
                            throw TypeError(".cordum.agent.v1.ContextHints.tags: array expected");
                        message.tags = [];
                        for (var i = 0; i < object.tags.length; ++i)
                            message.tags[i] = String(object.tags[i]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a ContextHints message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.ContextHints
                 * @static
                 * @param {cordum.agent.v1.ContextHints} message ContextHints
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
                 * @memberof cordum.agent.v1.ContextHints
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                ContextHints.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for ContextHints
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.ContextHints
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                ContextHints.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.ContextHints";
                };

                return ContextHints;
            })();

            v1.Budget = (function() {

                /**
                 * Properties of a Budget.
                 * @memberof cordum.agent.v1
                 * @interface IBudget
                 * @property {number|Long|null} [maxInputTokens] Budget maxInputTokens
                 * @property {number|Long|null} [maxOutputTokens] Budget maxOutputTokens
                 * @property {number|Long|null} [maxTotalTokens] Budget maxTotalTokens
                 * @property {number|Long|null} [deadlineMs] Budget deadlineMs
                 */

                /**
                 * Constructs a new Budget.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a Budget.
                 * @implements IBudget
                 * @constructor
                 * @param {cordum.agent.v1.IBudget=} [properties] Properties to set
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
                 * @memberof cordum.agent.v1.Budget
                 * @instance
                 */
                Budget.prototype.maxInputTokens = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * Budget maxOutputTokens.
                 * @member {number|Long} maxOutputTokens
                 * @memberof cordum.agent.v1.Budget
                 * @instance
                 */
                Budget.prototype.maxOutputTokens = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * Budget maxTotalTokens.
                 * @member {number|Long} maxTotalTokens
                 * @memberof cordum.agent.v1.Budget
                 * @instance
                 */
                Budget.prototype.maxTotalTokens = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * Budget deadlineMs.
                 * @member {number|Long} deadlineMs
                 * @memberof cordum.agent.v1.Budget
                 * @instance
                 */
                Budget.prototype.deadlineMs = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * Creates a new Budget instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.Budget
                 * @static
                 * @param {cordum.agent.v1.IBudget=} [properties] Properties to set
                 * @returns {cordum.agent.v1.Budget} Budget instance
                 */
                Budget.create = function create(properties) {
                    return new Budget(properties);
                };

                /**
                 * Encodes the specified Budget message. Does not implicitly {@link cordum.agent.v1.Budget.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.Budget
                 * @static
                 * @param {cordum.agent.v1.IBudget} message Budget message or plain object to encode
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
                 * Encodes the specified Budget message, length delimited. Does not implicitly {@link cordum.agent.v1.Budget.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.Budget
                 * @static
                 * @param {cordum.agent.v1.IBudget} message Budget message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                Budget.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a Budget message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.Budget
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.Budget} Budget
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                Budget.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.Budget();
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
                 * @memberof cordum.agent.v1.Budget
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.Budget} Budget
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
                 * @memberof cordum.agent.v1.Budget
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
                 * @memberof cordum.agent.v1.Budget
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.Budget} Budget
                 */
                Budget.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.Budget)
                        return object;
                    var message = new $root.cordum.agent.v1.Budget();
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
                 * @memberof cordum.agent.v1.Budget
                 * @static
                 * @param {cordum.agent.v1.Budget} message Budget
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
                 * @memberof cordum.agent.v1.Budget
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                Budget.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for Budget
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.Budget
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                Budget.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.Budget";
                };

                return Budget;
            })();

            /**
             * ActorType enum.
             * @name cordum.agent.v1.ActorType
             * @enum {number}
             * @property {number} ACTOR_TYPE_UNSPECIFIED=0 ACTOR_TYPE_UNSPECIFIED value
             * @property {number} ACTOR_TYPE_HUMAN=1 ACTOR_TYPE_HUMAN value
             * @property {number} ACTOR_TYPE_SERVICE=2 ACTOR_TYPE_SERVICE value
             */
            v1.ActorType = (function() {
                var valuesById = {}, values = Object.create(valuesById);
                values[valuesById[0] = "ACTOR_TYPE_UNSPECIFIED"] = 0;
                values[valuesById[1] = "ACTOR_TYPE_HUMAN"] = 1;
                values[valuesById[2] = "ACTOR_TYPE_SERVICE"] = 2;
                return values;
            })();

            v1.JobMetadata = (function() {

                /**
                 * Properties of a JobMetadata.
                 * @memberof cordum.agent.v1
                 * @interface IJobMetadata
                 * @property {string|null} [tenantId] JobMetadata tenantId
                 * @property {string|null} [actorId] JobMetadata actorId
                 * @property {cordum.agent.v1.ActorType|null} [actorType] JobMetadata actorType
                 * @property {string|null} [idempotencyKey] JobMetadata idempotencyKey
                 * @property {string|null} [capability] JobMetadata capability
                 * @property {Array.<string>|null} [riskTags] JobMetadata riskTags
                 * @property {Array.<string>|null} [requires] JobMetadata requires
                 * @property {string|null} [packId] JobMetadata packId
                 * @property {Object.<string,string>|null} [labels] JobMetadata labels
                 */

                /**
                 * Constructs a new JobMetadata.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a JobMetadata.
                 * @implements IJobMetadata
                 * @constructor
                 * @param {cordum.agent.v1.IJobMetadata=} [properties] Properties to set
                 */
                function JobMetadata(properties) {
                    this.riskTags = [];
                    this.requires = [];
                    this.labels = {};
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * JobMetadata tenantId.
                 * @member {string} tenantId
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 */
                JobMetadata.prototype.tenantId = "";

                /**
                 * JobMetadata actorId.
                 * @member {string} actorId
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 */
                JobMetadata.prototype.actorId = "";

                /**
                 * JobMetadata actorType.
                 * @member {cordum.agent.v1.ActorType} actorType
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 */
                JobMetadata.prototype.actorType = 0;

                /**
                 * JobMetadata idempotencyKey.
                 * @member {string} idempotencyKey
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 */
                JobMetadata.prototype.idempotencyKey = "";

                /**
                 * JobMetadata capability.
                 * @member {string} capability
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 */
                JobMetadata.prototype.capability = "";

                /**
                 * JobMetadata riskTags.
                 * @member {Array.<string>} riskTags
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 */
                JobMetadata.prototype.riskTags = $util.emptyArray;

                /**
                 * JobMetadata requires.
                 * @member {Array.<string>} requires
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 */
                JobMetadata.prototype.requires = $util.emptyArray;

                /**
                 * JobMetadata packId.
                 * @member {string} packId
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 */
                JobMetadata.prototype.packId = "";

                /**
                 * JobMetadata labels.
                 * @member {Object.<string,string>} labels
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 */
                JobMetadata.prototype.labels = $util.emptyObject;

                /**
                 * Creates a new JobMetadata instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.JobMetadata
                 * @static
                 * @param {cordum.agent.v1.IJobMetadata=} [properties] Properties to set
                 * @returns {cordum.agent.v1.JobMetadata} JobMetadata instance
                 */
                JobMetadata.create = function create(properties) {
                    return new JobMetadata(properties);
                };

                /**
                 * Encodes the specified JobMetadata message. Does not implicitly {@link cordum.agent.v1.JobMetadata.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.JobMetadata
                 * @static
                 * @param {cordum.agent.v1.IJobMetadata} message JobMetadata message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobMetadata.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.tenantId != null && Object.hasOwnProperty.call(message, "tenantId"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.tenantId);
                    if (message.actorId != null && Object.hasOwnProperty.call(message, "actorId"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.actorId);
                    if (message.actorType != null && Object.hasOwnProperty.call(message, "actorType"))
                        writer.uint32(/* id 3, wireType 0 =*/24).int32(message.actorType);
                    if (message.idempotencyKey != null && Object.hasOwnProperty.call(message, "idempotencyKey"))
                        writer.uint32(/* id 4, wireType 2 =*/34).string(message.idempotencyKey);
                    if (message.capability != null && Object.hasOwnProperty.call(message, "capability"))
                        writer.uint32(/* id 5, wireType 2 =*/42).string(message.capability);
                    if (message.riskTags != null && message.riskTags.length)
                        for (var i = 0; i < message.riskTags.length; ++i)
                            writer.uint32(/* id 6, wireType 2 =*/50).string(message.riskTags[i]);
                    if (message.requires != null && message.requires.length)
                        for (var i = 0; i < message.requires.length; ++i)
                            writer.uint32(/* id 7, wireType 2 =*/58).string(message.requires[i]);
                    if (message.packId != null && Object.hasOwnProperty.call(message, "packId"))
                        writer.uint32(/* id 8, wireType 2 =*/66).string(message.packId);
                    if (message.labels != null && Object.hasOwnProperty.call(message, "labels"))
                        for (var keys = Object.keys(message.labels), i = 0; i < keys.length; ++i)
                            writer.uint32(/* id 9, wireType 2 =*/74).fork().uint32(/* id 1, wireType 2 =*/10).string(keys[i]).uint32(/* id 2, wireType 2 =*/18).string(message.labels[keys[i]]).ldelim();
                    return writer;
                };

                /**
                 * Encodes the specified JobMetadata message, length delimited. Does not implicitly {@link cordum.agent.v1.JobMetadata.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.JobMetadata
                 * @static
                 * @param {cordum.agent.v1.IJobMetadata} message JobMetadata message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobMetadata.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a JobMetadata message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.JobMetadata
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.JobMetadata} JobMetadata
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobMetadata.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.JobMetadata(), key, value;
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.tenantId = reader.string();
                                break;
                            }
                        case 2: {
                                message.actorId = reader.string();
                                break;
                            }
                        case 3: {
                                message.actorType = reader.int32();
                                break;
                            }
                        case 4: {
                                message.idempotencyKey = reader.string();
                                break;
                            }
                        case 5: {
                                message.capability = reader.string();
                                break;
                            }
                        case 6: {
                                if (!(message.riskTags && message.riskTags.length))
                                    message.riskTags = [];
                                message.riskTags.push(reader.string());
                                break;
                            }
                        case 7: {
                                if (!(message.requires && message.requires.length))
                                    message.requires = [];
                                message.requires.push(reader.string());
                                break;
                            }
                        case 8: {
                                message.packId = reader.string();
                                break;
                            }
                        case 9: {
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
                 * Decodes a JobMetadata message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.JobMetadata
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.JobMetadata} JobMetadata
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobMetadata.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a JobMetadata message.
                 * @function verify
                 * @memberof cordum.agent.v1.JobMetadata
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                JobMetadata.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.tenantId != null && message.hasOwnProperty("tenantId"))
                        if (!$util.isString(message.tenantId))
                            return "tenantId: string expected";
                    if (message.actorId != null && message.hasOwnProperty("actorId"))
                        if (!$util.isString(message.actorId))
                            return "actorId: string expected";
                    if (message.actorType != null && message.hasOwnProperty("actorType"))
                        switch (message.actorType) {
                        default:
                            return "actorType: enum value expected";
                        case 0:
                        case 1:
                        case 2:
                            break;
                        }
                    if (message.idempotencyKey != null && message.hasOwnProperty("idempotencyKey"))
                        if (!$util.isString(message.idempotencyKey))
                            return "idempotencyKey: string expected";
                    if (message.capability != null && message.hasOwnProperty("capability"))
                        if (!$util.isString(message.capability))
                            return "capability: string expected";
                    if (message.riskTags != null && message.hasOwnProperty("riskTags")) {
                        if (!Array.isArray(message.riskTags))
                            return "riskTags: array expected";
                        for (var i = 0; i < message.riskTags.length; ++i)
                            if (!$util.isString(message.riskTags[i]))
                                return "riskTags: string[] expected";
                    }
                    if (message.requires != null && message.hasOwnProperty("requires")) {
                        if (!Array.isArray(message.requires))
                            return "requires: array expected";
                        for (var i = 0; i < message.requires.length; ++i)
                            if (!$util.isString(message.requires[i]))
                                return "requires: string[] expected";
                    }
                    if (message.packId != null && message.hasOwnProperty("packId"))
                        if (!$util.isString(message.packId))
                            return "packId: string expected";
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
                 * Creates a JobMetadata message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.JobMetadata
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.JobMetadata} JobMetadata
                 */
                JobMetadata.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.JobMetadata)
                        return object;
                    var message = new $root.cordum.agent.v1.JobMetadata();
                    if (object.tenantId != null)
                        message.tenantId = String(object.tenantId);
                    if (object.actorId != null)
                        message.actorId = String(object.actorId);
                    switch (object.actorType) {
                    default:
                        if (typeof object.actorType === "number") {
                            message.actorType = object.actorType;
                            break;
                        }
                        break;
                    case "ACTOR_TYPE_UNSPECIFIED":
                    case 0:
                        message.actorType = 0;
                        break;
                    case "ACTOR_TYPE_HUMAN":
                    case 1:
                        message.actorType = 1;
                        break;
                    case "ACTOR_TYPE_SERVICE":
                    case 2:
                        message.actorType = 2;
                        break;
                    }
                    if (object.idempotencyKey != null)
                        message.idempotencyKey = String(object.idempotencyKey);
                    if (object.capability != null)
                        message.capability = String(object.capability);
                    if (object.riskTags) {
                        if (!Array.isArray(object.riskTags))
                            throw TypeError(".cordum.agent.v1.JobMetadata.riskTags: array expected");
                        message.riskTags = [];
                        for (var i = 0; i < object.riskTags.length; ++i)
                            message.riskTags[i] = String(object.riskTags[i]);
                    }
                    if (object.requires) {
                        if (!Array.isArray(object.requires))
                            throw TypeError(".cordum.agent.v1.JobMetadata.requires: array expected");
                        message.requires = [];
                        for (var i = 0; i < object.requires.length; ++i)
                            message.requires[i] = String(object.requires[i]);
                    }
                    if (object.packId != null)
                        message.packId = String(object.packId);
                    if (object.labels) {
                        if (typeof object.labels !== "object")
                            throw TypeError(".cordum.agent.v1.JobMetadata.labels: object expected");
                        message.labels = {};
                        for (var keys = Object.keys(object.labels), i = 0; i < keys.length; ++i)
                            message.labels[keys[i]] = String(object.labels[keys[i]]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a JobMetadata message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.JobMetadata
                 * @static
                 * @param {cordum.agent.v1.JobMetadata} message JobMetadata
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                JobMetadata.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults) {
                        object.riskTags = [];
                        object.requires = [];
                    }
                    if (options.objects || options.defaults)
                        object.labels = {};
                    if (options.defaults) {
                        object.tenantId = "";
                        object.actorId = "";
                        object.actorType = options.enums === String ? "ACTOR_TYPE_UNSPECIFIED" : 0;
                        object.idempotencyKey = "";
                        object.capability = "";
                        object.packId = "";
                    }
                    if (message.tenantId != null && message.hasOwnProperty("tenantId"))
                        object.tenantId = message.tenantId;
                    if (message.actorId != null && message.hasOwnProperty("actorId"))
                        object.actorId = message.actorId;
                    if (message.actorType != null && message.hasOwnProperty("actorType"))
                        object.actorType = options.enums === String ? $root.cordum.agent.v1.ActorType[message.actorType] === undefined ? message.actorType : $root.cordum.agent.v1.ActorType[message.actorType] : message.actorType;
                    if (message.idempotencyKey != null && message.hasOwnProperty("idempotencyKey"))
                        object.idempotencyKey = message.idempotencyKey;
                    if (message.capability != null && message.hasOwnProperty("capability"))
                        object.capability = message.capability;
                    if (message.riskTags && message.riskTags.length) {
                        object.riskTags = [];
                        for (var j = 0; j < message.riskTags.length; ++j)
                            object.riskTags[j] = message.riskTags[j];
                    }
                    if (message.requires && message.requires.length) {
                        object.requires = [];
                        for (var j = 0; j < message.requires.length; ++j)
                            object.requires[j] = message.requires[j];
                    }
                    if (message.packId != null && message.hasOwnProperty("packId"))
                        object.packId = message.packId;
                    var keys2;
                    if (message.labels && (keys2 = Object.keys(message.labels)).length) {
                        object.labels = {};
                        for (var j = 0; j < keys2.length; ++j)
                            object.labels[keys2[j]] = message.labels[keys2[j]];
                    }
                    return object;
                };

                /**
                 * Converts this JobMetadata to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.JobMetadata
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                JobMetadata.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for JobMetadata
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.JobMetadata
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                JobMetadata.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.JobMetadata";
                };

                return JobMetadata;
            })();

            v1.JobRequest = (function() {

                /**
                 * Properties of a JobRequest.
                 * @memberof cordum.agent.v1
                 * @interface IJobRequest
                 * @property {string|null} [jobId] JobRequest jobId
                 * @property {string|null} [topic] JobRequest topic
                 * @property {cordum.agent.v1.JobPriority|null} [priority] JobRequest priority
                 * @property {string|null} [contextPtr] JobRequest contextPtr
                 * @property {string|null} [adapterId] JobRequest adapterId
                 * @property {Object.<string,string>|null} [env] JobRequest env
                 * @property {string|null} [parentJobId] JobRequest parentJobId
                 * @property {string|null} [workflowId] JobRequest workflowId
                 * @property {number|null} [stepIndex] JobRequest stepIndex
                 * @property {string|null} [memoryId] JobRequest memoryId
                 * @property {cordum.agent.v1.IContextHints|null} [contextHints] JobRequest contextHints
                 * @property {cordum.agent.v1.IBudget|null} [budget] JobRequest budget
                 * @property {string|null} [tenantId] JobRequest tenantId
                 * @property {string|null} [principalId] JobRequest principalId
                 * @property {Object.<string,string>|null} [labels] JobRequest labels
                 * @property {cordum.agent.v1.IJobMetadata|null} [meta] JobRequest meta
                 */

                /**
                 * Constructs a new JobRequest.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a JobRequest.
                 * @implements IJobRequest
                 * @constructor
                 * @param {cordum.agent.v1.IJobRequest=} [properties] Properties to set
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
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.jobId = "";

                /**
                 * JobRequest topic.
                 * @member {string} topic
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.topic = "";

                /**
                 * JobRequest priority.
                 * @member {cordum.agent.v1.JobPriority} priority
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.priority = 0;

                /**
                 * JobRequest contextPtr.
                 * @member {string} contextPtr
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.contextPtr = "";

                /**
                 * JobRequest adapterId.
                 * @member {string} adapterId
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.adapterId = "";

                /**
                 * JobRequest env.
                 * @member {Object.<string,string>} env
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.env = $util.emptyObject;

                /**
                 * JobRequest parentJobId.
                 * @member {string} parentJobId
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.parentJobId = "";

                /**
                 * JobRequest workflowId.
                 * @member {string} workflowId
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.workflowId = "";

                /**
                 * JobRequest stepIndex.
                 * @member {number} stepIndex
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.stepIndex = 0;

                /**
                 * JobRequest memoryId.
                 * @member {string} memoryId
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.memoryId = "";

                /**
                 * JobRequest contextHints.
                 * @member {cordum.agent.v1.IContextHints|null|undefined} contextHints
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.contextHints = null;

                /**
                 * JobRequest budget.
                 * @member {cordum.agent.v1.IBudget|null|undefined} budget
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.budget = null;

                /**
                 * JobRequest tenantId.
                 * @member {string} tenantId
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.tenantId = "";

                /**
                 * JobRequest principalId.
                 * @member {string} principalId
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.principalId = "";

                /**
                 * JobRequest labels.
                 * @member {Object.<string,string>} labels
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.labels = $util.emptyObject;

                /**
                 * JobRequest meta.
                 * @member {cordum.agent.v1.IJobMetadata|null|undefined} meta
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 */
                JobRequest.prototype.meta = null;

                /**
                 * Creates a new JobRequest instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.JobRequest
                 * @static
                 * @param {cordum.agent.v1.IJobRequest=} [properties] Properties to set
                 * @returns {cordum.agent.v1.JobRequest} JobRequest instance
                 */
                JobRequest.create = function create(properties) {
                    return new JobRequest(properties);
                };

                /**
                 * Encodes the specified JobRequest message. Does not implicitly {@link cordum.agent.v1.JobRequest.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.JobRequest
                 * @static
                 * @param {cordum.agent.v1.IJobRequest} message JobRequest message or plain object to encode
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
                        $root.cordum.agent.v1.ContextHints.encode(message.contextHints, writer.uint32(/* id 11, wireType 2 =*/90).fork()).ldelim();
                    if (message.budget != null && Object.hasOwnProperty.call(message, "budget"))
                        $root.cordum.agent.v1.Budget.encode(message.budget, writer.uint32(/* id 12, wireType 2 =*/98).fork()).ldelim();
                    if (message.tenantId != null && Object.hasOwnProperty.call(message, "tenantId"))
                        writer.uint32(/* id 13, wireType 2 =*/106).string(message.tenantId);
                    if (message.principalId != null && Object.hasOwnProperty.call(message, "principalId"))
                        writer.uint32(/* id 14, wireType 2 =*/114).string(message.principalId);
                    if (message.labels != null && Object.hasOwnProperty.call(message, "labels"))
                        for (var keys = Object.keys(message.labels), i = 0; i < keys.length; ++i)
                            writer.uint32(/* id 15, wireType 2 =*/122).fork().uint32(/* id 1, wireType 2 =*/10).string(keys[i]).uint32(/* id 2, wireType 2 =*/18).string(message.labels[keys[i]]).ldelim();
                    if (message.meta != null && Object.hasOwnProperty.call(message, "meta"))
                        $root.cordum.agent.v1.JobMetadata.encode(message.meta, writer.uint32(/* id 16, wireType 2 =*/130).fork()).ldelim();
                    return writer;
                };

                /**
                 * Encodes the specified JobRequest message, length delimited. Does not implicitly {@link cordum.agent.v1.JobRequest.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.JobRequest
                 * @static
                 * @param {cordum.agent.v1.IJobRequest} message JobRequest message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobRequest.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a JobRequest message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.JobRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.JobRequest} JobRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobRequest.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.JobRequest(), key, value;
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
                                message.contextHints = $root.cordum.agent.v1.ContextHints.decode(reader, reader.uint32());
                                break;
                            }
                        case 12: {
                                message.budget = $root.cordum.agent.v1.Budget.decode(reader, reader.uint32());
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
                        case 16: {
                                message.meta = $root.cordum.agent.v1.JobMetadata.decode(reader, reader.uint32());
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
                 * @memberof cordum.agent.v1.JobRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.JobRequest} JobRequest
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
                 * @memberof cordum.agent.v1.JobRequest
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
                        var error = $root.cordum.agent.v1.ContextHints.verify(message.contextHints);
                        if (error)
                            return "contextHints." + error;
                    }
                    if (message.budget != null && message.hasOwnProperty("budget")) {
                        var error = $root.cordum.agent.v1.Budget.verify(message.budget);
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
                    if (message.meta != null && message.hasOwnProperty("meta")) {
                        var error = $root.cordum.agent.v1.JobMetadata.verify(message.meta);
                        if (error)
                            return "meta." + error;
                    }
                    return null;
                };

                /**
                 * Creates a JobRequest message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.JobRequest
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.JobRequest} JobRequest
                 */
                JobRequest.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.JobRequest)
                        return object;
                    var message = new $root.cordum.agent.v1.JobRequest();
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
                            throw TypeError(".cordum.agent.v1.JobRequest.env: object expected");
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
                            throw TypeError(".cordum.agent.v1.JobRequest.contextHints: object expected");
                        message.contextHints = $root.cordum.agent.v1.ContextHints.fromObject(object.contextHints);
                    }
                    if (object.budget != null) {
                        if (typeof object.budget !== "object")
                            throw TypeError(".cordum.agent.v1.JobRequest.budget: object expected");
                        message.budget = $root.cordum.agent.v1.Budget.fromObject(object.budget);
                    }
                    if (object.tenantId != null)
                        message.tenantId = String(object.tenantId);
                    if (object.principalId != null)
                        message.principalId = String(object.principalId);
                    if (object.labels) {
                        if (typeof object.labels !== "object")
                            throw TypeError(".cordum.agent.v1.JobRequest.labels: object expected");
                        message.labels = {};
                        for (var keys = Object.keys(object.labels), i = 0; i < keys.length; ++i)
                            message.labels[keys[i]] = String(object.labels[keys[i]]);
                    }
                    if (object.meta != null) {
                        if (typeof object.meta !== "object")
                            throw TypeError(".cordum.agent.v1.JobRequest.meta: object expected");
                        message.meta = $root.cordum.agent.v1.JobMetadata.fromObject(object.meta);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a JobRequest message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.JobRequest
                 * @static
                 * @param {cordum.agent.v1.JobRequest} message JobRequest
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
                        object.meta = null;
                    }
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        object.jobId = message.jobId;
                    if (message.topic != null && message.hasOwnProperty("topic"))
                        object.topic = message.topic;
                    if (message.priority != null && message.hasOwnProperty("priority"))
                        object.priority = options.enums === String ? $root.cordum.agent.v1.JobPriority[message.priority] === undefined ? message.priority : $root.cordum.agent.v1.JobPriority[message.priority] : message.priority;
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
                        object.contextHints = $root.cordum.agent.v1.ContextHints.toObject(message.contextHints, options);
                    if (message.budget != null && message.hasOwnProperty("budget"))
                        object.budget = $root.cordum.agent.v1.Budget.toObject(message.budget, options);
                    if (message.tenantId != null && message.hasOwnProperty("tenantId"))
                        object.tenantId = message.tenantId;
                    if (message.principalId != null && message.hasOwnProperty("principalId"))
                        object.principalId = message.principalId;
                    if (message.labels && (keys2 = Object.keys(message.labels)).length) {
                        object.labels = {};
                        for (var j = 0; j < keys2.length; ++j)
                            object.labels[keys2[j]] = message.labels[keys2[j]];
                    }
                    if (message.meta != null && message.hasOwnProperty("meta"))
                        object.meta = $root.cordum.agent.v1.JobMetadata.toObject(message.meta, options);
                    return object;
                };

                /**
                 * Converts this JobRequest to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.JobRequest
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                JobRequest.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for JobRequest
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.JobRequest
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                JobRequest.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.JobRequest";
                };

                return JobRequest;
            })();

            v1.JobResult = (function() {

                /**
                 * Properties of a JobResult.
                 * @memberof cordum.agent.v1
                 * @interface IJobResult
                 * @property {string|null} [jobId] JobResult jobId
                 * @property {cordum.agent.v1.JobStatus|null} [status] JobResult status
                 * @property {string|null} [resultPtr] JobResult resultPtr
                 * @property {string|null} [workerId] JobResult workerId
                 * @property {number|Long|null} [executionMs] JobResult executionMs
                 * @property {string|null} [errorCode] JobResult errorCode
                 * @property {string|null} [errorMessage] JobResult errorMessage
                 * @property {Array.<string>|null} [artifactPtrs] JobResult artifactPtrs
                 */

                /**
                 * Constructs a new JobResult.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a JobResult.
                 * @implements IJobResult
                 * @constructor
                 * @param {cordum.agent.v1.IJobResult=} [properties] Properties to set
                 */
                function JobResult(properties) {
                    this.artifactPtrs = [];
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * JobResult jobId.
                 * @member {string} jobId
                 * @memberof cordum.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.jobId = "";

                /**
                 * JobResult status.
                 * @member {cordum.agent.v1.JobStatus} status
                 * @memberof cordum.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.status = 0;

                /**
                 * JobResult resultPtr.
                 * @member {string} resultPtr
                 * @memberof cordum.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.resultPtr = "";

                /**
                 * JobResult workerId.
                 * @member {string} workerId
                 * @memberof cordum.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.workerId = "";

                /**
                 * JobResult executionMs.
                 * @member {number|Long} executionMs
                 * @memberof cordum.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.executionMs = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * JobResult errorCode.
                 * @member {string} errorCode
                 * @memberof cordum.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.errorCode = "";

                /**
                 * JobResult errorMessage.
                 * @member {string} errorMessage
                 * @memberof cordum.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.errorMessage = "";

                /**
                 * JobResult artifactPtrs.
                 * @member {Array.<string>} artifactPtrs
                 * @memberof cordum.agent.v1.JobResult
                 * @instance
                 */
                JobResult.prototype.artifactPtrs = $util.emptyArray;

                /**
                 * Creates a new JobResult instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.JobResult
                 * @static
                 * @param {cordum.agent.v1.IJobResult=} [properties] Properties to set
                 * @returns {cordum.agent.v1.JobResult} JobResult instance
                 */
                JobResult.create = function create(properties) {
                    return new JobResult(properties);
                };

                /**
                 * Encodes the specified JobResult message. Does not implicitly {@link cordum.agent.v1.JobResult.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.JobResult
                 * @static
                 * @param {cordum.agent.v1.IJobResult} message JobResult message or plain object to encode
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
                    if (message.artifactPtrs != null && message.artifactPtrs.length)
                        for (var i = 0; i < message.artifactPtrs.length; ++i)
                            writer.uint32(/* id 8, wireType 2 =*/66).string(message.artifactPtrs[i]);
                    return writer;
                };

                /**
                 * Encodes the specified JobResult message, length delimited. Does not implicitly {@link cordum.agent.v1.JobResult.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.JobResult
                 * @static
                 * @param {cordum.agent.v1.IJobResult} message JobResult message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobResult.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a JobResult message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.JobResult
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.JobResult} JobResult
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobResult.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.JobResult();
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
                        case 8: {
                                if (!(message.artifactPtrs && message.artifactPtrs.length))
                                    message.artifactPtrs = [];
                                message.artifactPtrs.push(reader.string());
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
                 * @memberof cordum.agent.v1.JobResult
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.JobResult} JobResult
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
                 * @memberof cordum.agent.v1.JobResult
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
                    if (message.artifactPtrs != null && message.hasOwnProperty("artifactPtrs")) {
                        if (!Array.isArray(message.artifactPtrs))
                            return "artifactPtrs: array expected";
                        for (var i = 0; i < message.artifactPtrs.length; ++i)
                            if (!$util.isString(message.artifactPtrs[i]))
                                return "artifactPtrs: string[] expected";
                    }
                    return null;
                };

                /**
                 * Creates a JobResult message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.JobResult
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.JobResult} JobResult
                 */
                JobResult.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.JobResult)
                        return object;
                    var message = new $root.cordum.agent.v1.JobResult();
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
                    if (object.artifactPtrs) {
                        if (!Array.isArray(object.artifactPtrs))
                            throw TypeError(".cordum.agent.v1.JobResult.artifactPtrs: array expected");
                        message.artifactPtrs = [];
                        for (var i = 0; i < object.artifactPtrs.length; ++i)
                            message.artifactPtrs[i] = String(object.artifactPtrs[i]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a JobResult message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.JobResult
                 * @static
                 * @param {cordum.agent.v1.JobResult} message JobResult
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                JobResult.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults)
                        object.artifactPtrs = [];
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
                        object.status = options.enums === String ? $root.cordum.agent.v1.JobStatus[message.status] === undefined ? message.status : $root.cordum.agent.v1.JobStatus[message.status] : message.status;
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
                    if (message.artifactPtrs && message.artifactPtrs.length) {
                        object.artifactPtrs = [];
                        for (var j = 0; j < message.artifactPtrs.length; ++j)
                            object.artifactPtrs[j] = message.artifactPtrs[j];
                    }
                    return object;
                };

                /**
                 * Converts this JobResult to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.JobResult
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                JobResult.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for JobResult
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.JobResult
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                JobResult.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.JobResult";
                };

                return JobResult;
            })();

            v1.JobProgress = (function() {

                /**
                 * Properties of a JobProgress.
                 * @memberof cordum.agent.v1
                 * @interface IJobProgress
                 * @property {string|null} [jobId] JobProgress jobId
                 * @property {string|null} [stepId] JobProgress stepId
                 * @property {number|null} [percent] JobProgress percent
                 * @property {string|null} [message] JobProgress message
                 * @property {string|null} [resultPtr] JobProgress resultPtr
                 * @property {Array.<string>|null} [artifactPtrs] JobProgress artifactPtrs
                 * @property {cordum.agent.v1.JobStatus|null} [status] JobProgress status
                 */

                /**
                 * Constructs a new JobProgress.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a JobProgress.
                 * @implements IJobProgress
                 * @constructor
                 * @param {cordum.agent.v1.IJobProgress=} [properties] Properties to set
                 */
                function JobProgress(properties) {
                    this.artifactPtrs = [];
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * JobProgress jobId.
                 * @member {string} jobId
                 * @memberof cordum.agent.v1.JobProgress
                 * @instance
                 */
                JobProgress.prototype.jobId = "";

                /**
                 * JobProgress stepId.
                 * @member {string} stepId
                 * @memberof cordum.agent.v1.JobProgress
                 * @instance
                 */
                JobProgress.prototype.stepId = "";

                /**
                 * JobProgress percent.
                 * @member {number} percent
                 * @memberof cordum.agent.v1.JobProgress
                 * @instance
                 */
                JobProgress.prototype.percent = 0;

                /**
                 * JobProgress message.
                 * @member {string} message
                 * @memberof cordum.agent.v1.JobProgress
                 * @instance
                 */
                JobProgress.prototype.message = "";

                /**
                 * JobProgress resultPtr.
                 * @member {string} resultPtr
                 * @memberof cordum.agent.v1.JobProgress
                 * @instance
                 */
                JobProgress.prototype.resultPtr = "";

                /**
                 * JobProgress artifactPtrs.
                 * @member {Array.<string>} artifactPtrs
                 * @memberof cordum.agent.v1.JobProgress
                 * @instance
                 */
                JobProgress.prototype.artifactPtrs = $util.emptyArray;

                /**
                 * JobProgress status.
                 * @member {cordum.agent.v1.JobStatus} status
                 * @memberof cordum.agent.v1.JobProgress
                 * @instance
                 */
                JobProgress.prototype.status = 0;

                /**
                 * Creates a new JobProgress instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.JobProgress
                 * @static
                 * @param {cordum.agent.v1.IJobProgress=} [properties] Properties to set
                 * @returns {cordum.agent.v1.JobProgress} JobProgress instance
                 */
                JobProgress.create = function create(properties) {
                    return new JobProgress(properties);
                };

                /**
                 * Encodes the specified JobProgress message. Does not implicitly {@link cordum.agent.v1.JobProgress.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.JobProgress
                 * @static
                 * @param {cordum.agent.v1.IJobProgress} message JobProgress message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobProgress.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.jobId != null && Object.hasOwnProperty.call(message, "jobId"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.jobId);
                    if (message.stepId != null && Object.hasOwnProperty.call(message, "stepId"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.stepId);
                    if (message.percent != null && Object.hasOwnProperty.call(message, "percent"))
                        writer.uint32(/* id 3, wireType 0 =*/24).int32(message.percent);
                    if (message.message != null && Object.hasOwnProperty.call(message, "message"))
                        writer.uint32(/* id 4, wireType 2 =*/34).string(message.message);
                    if (message.resultPtr != null && Object.hasOwnProperty.call(message, "resultPtr"))
                        writer.uint32(/* id 5, wireType 2 =*/42).string(message.resultPtr);
                    if (message.artifactPtrs != null && message.artifactPtrs.length)
                        for (var i = 0; i < message.artifactPtrs.length; ++i)
                            writer.uint32(/* id 6, wireType 2 =*/50).string(message.artifactPtrs[i]);
                    if (message.status != null && Object.hasOwnProperty.call(message, "status"))
                        writer.uint32(/* id 7, wireType 0 =*/56).int32(message.status);
                    return writer;
                };

                /**
                 * Encodes the specified JobProgress message, length delimited. Does not implicitly {@link cordum.agent.v1.JobProgress.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.JobProgress
                 * @static
                 * @param {cordum.agent.v1.IJobProgress} message JobProgress message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobProgress.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a JobProgress message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.JobProgress
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.JobProgress} JobProgress
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobProgress.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.JobProgress();
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
                                message.stepId = reader.string();
                                break;
                            }
                        case 3: {
                                message.percent = reader.int32();
                                break;
                            }
                        case 4: {
                                message.message = reader.string();
                                break;
                            }
                        case 5: {
                                message.resultPtr = reader.string();
                                break;
                            }
                        case 6: {
                                if (!(message.artifactPtrs && message.artifactPtrs.length))
                                    message.artifactPtrs = [];
                                message.artifactPtrs.push(reader.string());
                                break;
                            }
                        case 7: {
                                message.status = reader.int32();
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
                 * Decodes a JobProgress message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.JobProgress
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.JobProgress} JobProgress
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobProgress.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a JobProgress message.
                 * @function verify
                 * @memberof cordum.agent.v1.JobProgress
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                JobProgress.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        if (!$util.isString(message.jobId))
                            return "jobId: string expected";
                    if (message.stepId != null && message.hasOwnProperty("stepId"))
                        if (!$util.isString(message.stepId))
                            return "stepId: string expected";
                    if (message.percent != null && message.hasOwnProperty("percent"))
                        if (!$util.isInteger(message.percent))
                            return "percent: integer expected";
                    if (message.message != null && message.hasOwnProperty("message"))
                        if (!$util.isString(message.message))
                            return "message: string expected";
                    if (message.resultPtr != null && message.hasOwnProperty("resultPtr"))
                        if (!$util.isString(message.resultPtr))
                            return "resultPtr: string expected";
                    if (message.artifactPtrs != null && message.hasOwnProperty("artifactPtrs")) {
                        if (!Array.isArray(message.artifactPtrs))
                            return "artifactPtrs: array expected";
                        for (var i = 0; i < message.artifactPtrs.length; ++i)
                            if (!$util.isString(message.artifactPtrs[i]))
                                return "artifactPtrs: string[] expected";
                    }
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
                    return null;
                };

                /**
                 * Creates a JobProgress message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.JobProgress
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.JobProgress} JobProgress
                 */
                JobProgress.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.JobProgress)
                        return object;
                    var message = new $root.cordum.agent.v1.JobProgress();
                    if (object.jobId != null)
                        message.jobId = String(object.jobId);
                    if (object.stepId != null)
                        message.stepId = String(object.stepId);
                    if (object.percent != null)
                        message.percent = object.percent | 0;
                    if (object.message != null)
                        message.message = String(object.message);
                    if (object.resultPtr != null)
                        message.resultPtr = String(object.resultPtr);
                    if (object.artifactPtrs) {
                        if (!Array.isArray(object.artifactPtrs))
                            throw TypeError(".cordum.agent.v1.JobProgress.artifactPtrs: array expected");
                        message.artifactPtrs = [];
                        for (var i = 0; i < object.artifactPtrs.length; ++i)
                            message.artifactPtrs[i] = String(object.artifactPtrs[i]);
                    }
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
                    return message;
                };

                /**
                 * Creates a plain object from a JobProgress message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.JobProgress
                 * @static
                 * @param {cordum.agent.v1.JobProgress} message JobProgress
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                JobProgress.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults)
                        object.artifactPtrs = [];
                    if (options.defaults) {
                        object.jobId = "";
                        object.stepId = "";
                        object.percent = 0;
                        object.message = "";
                        object.resultPtr = "";
                        object.status = options.enums === String ? "JOB_STATUS_UNSPECIFIED" : 0;
                    }
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        object.jobId = message.jobId;
                    if (message.stepId != null && message.hasOwnProperty("stepId"))
                        object.stepId = message.stepId;
                    if (message.percent != null && message.hasOwnProperty("percent"))
                        object.percent = message.percent;
                    if (message.message != null && message.hasOwnProperty("message"))
                        object.message = message.message;
                    if (message.resultPtr != null && message.hasOwnProperty("resultPtr"))
                        object.resultPtr = message.resultPtr;
                    if (message.artifactPtrs && message.artifactPtrs.length) {
                        object.artifactPtrs = [];
                        for (var j = 0; j < message.artifactPtrs.length; ++j)
                            object.artifactPtrs[j] = message.artifactPtrs[j];
                    }
                    if (message.status != null && message.hasOwnProperty("status"))
                        object.status = options.enums === String ? $root.cordum.agent.v1.JobStatus[message.status] === undefined ? message.status : $root.cordum.agent.v1.JobStatus[message.status] : message.status;
                    return object;
                };

                /**
                 * Converts this JobProgress to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.JobProgress
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                JobProgress.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for JobProgress
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.JobProgress
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                JobProgress.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.JobProgress";
                };

                return JobProgress;
            })();

            v1.JobCancel = (function() {

                /**
                 * Properties of a JobCancel.
                 * @memberof cordum.agent.v1
                 * @interface IJobCancel
                 * @property {string|null} [jobId] JobCancel jobId
                 * @property {string|null} [reason] JobCancel reason
                 * @property {string|null} [requestedBy] JobCancel requestedBy
                 */

                /**
                 * Constructs a new JobCancel.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a JobCancel.
                 * @implements IJobCancel
                 * @constructor
                 * @param {cordum.agent.v1.IJobCancel=} [properties] Properties to set
                 */
                function JobCancel(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * JobCancel jobId.
                 * @member {string} jobId
                 * @memberof cordum.agent.v1.JobCancel
                 * @instance
                 */
                JobCancel.prototype.jobId = "";

                /**
                 * JobCancel reason.
                 * @member {string} reason
                 * @memberof cordum.agent.v1.JobCancel
                 * @instance
                 */
                JobCancel.prototype.reason = "";

                /**
                 * JobCancel requestedBy.
                 * @member {string} requestedBy
                 * @memberof cordum.agent.v1.JobCancel
                 * @instance
                 */
                JobCancel.prototype.requestedBy = "";

                /**
                 * Creates a new JobCancel instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.JobCancel
                 * @static
                 * @param {cordum.agent.v1.IJobCancel=} [properties] Properties to set
                 * @returns {cordum.agent.v1.JobCancel} JobCancel instance
                 */
                JobCancel.create = function create(properties) {
                    return new JobCancel(properties);
                };

                /**
                 * Encodes the specified JobCancel message. Does not implicitly {@link cordum.agent.v1.JobCancel.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.JobCancel
                 * @static
                 * @param {cordum.agent.v1.IJobCancel} message JobCancel message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobCancel.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.jobId != null && Object.hasOwnProperty.call(message, "jobId"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.jobId);
                    if (message.reason != null && Object.hasOwnProperty.call(message, "reason"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.reason);
                    if (message.requestedBy != null && Object.hasOwnProperty.call(message, "requestedBy"))
                        writer.uint32(/* id 3, wireType 2 =*/26).string(message.requestedBy);
                    return writer;
                };

                /**
                 * Encodes the specified JobCancel message, length delimited. Does not implicitly {@link cordum.agent.v1.JobCancel.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.JobCancel
                 * @static
                 * @param {cordum.agent.v1.IJobCancel} message JobCancel message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                JobCancel.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a JobCancel message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.JobCancel
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.JobCancel} JobCancel
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobCancel.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.JobCancel();
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
                                message.reason = reader.string();
                                break;
                            }
                        case 3: {
                                message.requestedBy = reader.string();
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
                 * Decodes a JobCancel message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.JobCancel
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.JobCancel} JobCancel
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                JobCancel.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a JobCancel message.
                 * @function verify
                 * @memberof cordum.agent.v1.JobCancel
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                JobCancel.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        if (!$util.isString(message.jobId))
                            return "jobId: string expected";
                    if (message.reason != null && message.hasOwnProperty("reason"))
                        if (!$util.isString(message.reason))
                            return "reason: string expected";
                    if (message.requestedBy != null && message.hasOwnProperty("requestedBy"))
                        if (!$util.isString(message.requestedBy))
                            return "requestedBy: string expected";
                    return null;
                };

                /**
                 * Creates a JobCancel message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.JobCancel
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.JobCancel} JobCancel
                 */
                JobCancel.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.JobCancel)
                        return object;
                    var message = new $root.cordum.agent.v1.JobCancel();
                    if (object.jobId != null)
                        message.jobId = String(object.jobId);
                    if (object.reason != null)
                        message.reason = String(object.reason);
                    if (object.requestedBy != null)
                        message.requestedBy = String(object.requestedBy);
                    return message;
                };

                /**
                 * Creates a plain object from a JobCancel message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.JobCancel
                 * @static
                 * @param {cordum.agent.v1.JobCancel} message JobCancel
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                JobCancel.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.defaults) {
                        object.jobId = "";
                        object.reason = "";
                        object.requestedBy = "";
                    }
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        object.jobId = message.jobId;
                    if (message.reason != null && message.hasOwnProperty("reason"))
                        object.reason = message.reason;
                    if (message.requestedBy != null && message.hasOwnProperty("requestedBy"))
                        object.requestedBy = message.requestedBy;
                    return object;
                };

                /**
                 * Converts this JobCancel to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.JobCancel
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                JobCancel.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for JobCancel
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.JobCancel
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                JobCancel.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.JobCancel";
                };

                return JobCancel;
            })();

            v1.Heartbeat = (function() {

                /**
                 * Properties of a Heartbeat.
                 * @memberof cordum.agent.v1
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
                 * @property {Object.<string,string>|null} [labels] Heartbeat labels
                 */

                /**
                 * Constructs a new Heartbeat.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a Heartbeat.
                 * @implements IHeartbeat
                 * @constructor
                 * @param {cordum.agent.v1.IHeartbeat=} [properties] Properties to set
                 */
                function Heartbeat(properties) {
                    this.capabilities = [];
                    this.labels = {};
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * Heartbeat workerId.
                 * @member {string} workerId
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.workerId = "";

                /**
                 * Heartbeat region.
                 * @member {string} region
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.region = "";

                /**
                 * Heartbeat type.
                 * @member {string} type
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.type = "";

                /**
                 * Heartbeat cpuLoad.
                 * @member {number} cpuLoad
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.cpuLoad = 0;

                /**
                 * Heartbeat gpuUtilization.
                 * @member {number} gpuUtilization
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.gpuUtilization = 0;

                /**
                 * Heartbeat activeJobs.
                 * @member {number} activeJobs
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.activeJobs = 0;

                /**
                 * Heartbeat capabilities.
                 * @member {Array.<string>} capabilities
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.capabilities = $util.emptyArray;

                /**
                 * Heartbeat pool.
                 * @member {string} pool
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.pool = "";

                /**
                 * Heartbeat maxParallelJobs.
                 * @member {number} maxParallelJobs
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.maxParallelJobs = 0;

                /**
                 * Heartbeat labels.
                 * @member {Object.<string,string>} labels
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 */
                Heartbeat.prototype.labels = $util.emptyObject;

                /**
                 * Creates a new Heartbeat instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.Heartbeat
                 * @static
                 * @param {cordum.agent.v1.IHeartbeat=} [properties] Properties to set
                 * @returns {cordum.agent.v1.Heartbeat} Heartbeat instance
                 */
                Heartbeat.create = function create(properties) {
                    return new Heartbeat(properties);
                };

                /**
                 * Encodes the specified Heartbeat message. Does not implicitly {@link cordum.agent.v1.Heartbeat.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.Heartbeat
                 * @static
                 * @param {cordum.agent.v1.IHeartbeat} message Heartbeat message or plain object to encode
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
                    if (message.labels != null && Object.hasOwnProperty.call(message, "labels"))
                        for (var keys = Object.keys(message.labels), i = 0; i < keys.length; ++i)
                            writer.uint32(/* id 13, wireType 2 =*/106).fork().uint32(/* id 1, wireType 2 =*/10).string(keys[i]).uint32(/* id 2, wireType 2 =*/18).string(message.labels[keys[i]]).ldelim();
                    return writer;
                };

                /**
                 * Encodes the specified Heartbeat message, length delimited. Does not implicitly {@link cordum.agent.v1.Heartbeat.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.Heartbeat
                 * @static
                 * @param {cordum.agent.v1.IHeartbeat} message Heartbeat message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                Heartbeat.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a Heartbeat message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.Heartbeat
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.Heartbeat} Heartbeat
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                Heartbeat.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.Heartbeat(), key, value;
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
                        case 13: {
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
                 * Decodes a Heartbeat message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.Heartbeat
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.Heartbeat} Heartbeat
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
                 * @memberof cordum.agent.v1.Heartbeat
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
                 * Creates a Heartbeat message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.Heartbeat
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.Heartbeat} Heartbeat
                 */
                Heartbeat.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.Heartbeat)
                        return object;
                    var message = new $root.cordum.agent.v1.Heartbeat();
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
                            throw TypeError(".cordum.agent.v1.Heartbeat.capabilities: array expected");
                        message.capabilities = [];
                        for (var i = 0; i < object.capabilities.length; ++i)
                            message.capabilities[i] = String(object.capabilities[i]);
                    }
                    if (object.pool != null)
                        message.pool = String(object.pool);
                    if (object.maxParallelJobs != null)
                        message.maxParallelJobs = object.maxParallelJobs | 0;
                    if (object.labels) {
                        if (typeof object.labels !== "object")
                            throw TypeError(".cordum.agent.v1.Heartbeat.labels: object expected");
                        message.labels = {};
                        for (var keys = Object.keys(object.labels), i = 0; i < keys.length; ++i)
                            message.labels[keys[i]] = String(object.labels[keys[i]]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a Heartbeat message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.Heartbeat
                 * @static
                 * @param {cordum.agent.v1.Heartbeat} message Heartbeat
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                Heartbeat.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults)
                        object.capabilities = [];
                    if (options.objects || options.defaults)
                        object.labels = {};
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
                    var keys2;
                    if (message.labels && (keys2 = Object.keys(message.labels)).length) {
                        object.labels = {};
                        for (var j = 0; j < keys2.length; ++j)
                            object.labels[keys2[j]] = message.labels[keys2[j]];
                    }
                    return object;
                };

                /**
                 * Converts this Heartbeat to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.Heartbeat
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                Heartbeat.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for Heartbeat
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.Heartbeat
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                Heartbeat.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.Heartbeat";
                };

                return Heartbeat;
            })();

            /**
             * DecisionType enum.
             * @name cordum.agent.v1.DecisionType
             * @enum {number}
             * @property {number} DECISION_TYPE_UNSPECIFIED=0 DECISION_TYPE_UNSPECIFIED value
             * @property {number} DECISION_TYPE_ALLOW=1 DECISION_TYPE_ALLOW value
             * @property {number} DECISION_TYPE_DENY=2 DECISION_TYPE_DENY value
             * @property {number} DECISION_TYPE_REQUIRE_HUMAN=3 DECISION_TYPE_REQUIRE_HUMAN value
             * @property {number} DECISION_TYPE_THROTTLE=4 DECISION_TYPE_THROTTLE value
             * @property {number} DECISION_TYPE_ALLOW_WITH_CONSTRAINTS=5 DECISION_TYPE_ALLOW_WITH_CONSTRAINTS value
             */
            v1.DecisionType = (function() {
                var valuesById = {}, values = Object.create(valuesById);
                values[valuesById[0] = "DECISION_TYPE_UNSPECIFIED"] = 0;
                values[valuesById[1] = "DECISION_TYPE_ALLOW"] = 1;
                values[valuesById[2] = "DECISION_TYPE_DENY"] = 2;
                values[valuesById[3] = "DECISION_TYPE_REQUIRE_HUMAN"] = 3;
                values[valuesById[4] = "DECISION_TYPE_THROTTLE"] = 4;
                values[valuesById[5] = "DECISION_TYPE_ALLOW_WITH_CONSTRAINTS"] = 5;
                return values;
            })();

            v1.PolicyCheckRequest = (function() {

                /**
                 * Properties of a PolicyCheckRequest.
                 * @memberof cordum.agent.v1
                 * @interface IPolicyCheckRequest
                 * @property {string|null} [jobId] PolicyCheckRequest jobId
                 * @property {string|null} [topic] PolicyCheckRequest topic
                 * @property {string|null} [tenant] PolicyCheckRequest tenant
                 * @property {cordum.agent.v1.JobPriority|null} [priority] PolicyCheckRequest priority
                 * @property {number|null} [estimatedCost] PolicyCheckRequest estimatedCost
                 * @property {cordum.agent.v1.IBudget|null} [budget] PolicyCheckRequest budget
                 * @property {string|null} [principalId] PolicyCheckRequest principalId
                 * @property {Object.<string,string>|null} [labels] PolicyCheckRequest labels
                 * @property {string|null} [memoryId] PolicyCheckRequest memoryId
                 * @property {Uint8Array|null} [effectiveConfig] PolicyCheckRequest effectiveConfig
                 * @property {cordum.agent.v1.IJobMetadata|null} [meta] PolicyCheckRequest meta
                 */

                /**
                 * Constructs a new PolicyCheckRequest.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a PolicyCheckRequest.
                 * @implements IPolicyCheckRequest
                 * @constructor
                 * @param {cordum.agent.v1.IPolicyCheckRequest=} [properties] Properties to set
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
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.jobId = "";

                /**
                 * PolicyCheckRequest topic.
                 * @member {string} topic
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.topic = "";

                /**
                 * PolicyCheckRequest tenant.
                 * @member {string} tenant
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.tenant = "";

                /**
                 * PolicyCheckRequest priority.
                 * @member {cordum.agent.v1.JobPriority} priority
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.priority = 0;

                /**
                 * PolicyCheckRequest estimatedCost.
                 * @member {number} estimatedCost
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.estimatedCost = 0;

                /**
                 * PolicyCheckRequest budget.
                 * @member {cordum.agent.v1.IBudget|null|undefined} budget
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.budget = null;

                /**
                 * PolicyCheckRequest principalId.
                 * @member {string} principalId
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.principalId = "";

                /**
                 * PolicyCheckRequest labels.
                 * @member {Object.<string,string>} labels
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.labels = $util.emptyObject;

                /**
                 * PolicyCheckRequest memoryId.
                 * @member {string} memoryId
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.memoryId = "";

                /**
                 * PolicyCheckRequest effectiveConfig.
                 * @member {Uint8Array} effectiveConfig
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.effectiveConfig = $util.newBuffer([]);

                /**
                 * PolicyCheckRequest meta.
                 * @member {cordum.agent.v1.IJobMetadata|null|undefined} meta
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 */
                PolicyCheckRequest.prototype.meta = null;

                /**
                 * Creates a new PolicyCheckRequest instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {cordum.agent.v1.IPolicyCheckRequest=} [properties] Properties to set
                 * @returns {cordum.agent.v1.PolicyCheckRequest} PolicyCheckRequest instance
                 */
                PolicyCheckRequest.create = function create(properties) {
                    return new PolicyCheckRequest(properties);
                };

                /**
                 * Encodes the specified PolicyCheckRequest message. Does not implicitly {@link cordum.agent.v1.PolicyCheckRequest.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {cordum.agent.v1.IPolicyCheckRequest} message PolicyCheckRequest message or plain object to encode
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
                        $root.cordum.agent.v1.Budget.encode(message.budget, writer.uint32(/* id 6, wireType 2 =*/50).fork()).ldelim();
                    if (message.principalId != null && Object.hasOwnProperty.call(message, "principalId"))
                        writer.uint32(/* id 7, wireType 2 =*/58).string(message.principalId);
                    if (message.labels != null && Object.hasOwnProperty.call(message, "labels"))
                        for (var keys = Object.keys(message.labels), i = 0; i < keys.length; ++i)
                            writer.uint32(/* id 8, wireType 2 =*/66).fork().uint32(/* id 1, wireType 2 =*/10).string(keys[i]).uint32(/* id 2, wireType 2 =*/18).string(message.labels[keys[i]]).ldelim();
                    if (message.memoryId != null && Object.hasOwnProperty.call(message, "memoryId"))
                        writer.uint32(/* id 9, wireType 2 =*/74).string(message.memoryId);
                    if (message.effectiveConfig != null && Object.hasOwnProperty.call(message, "effectiveConfig"))
                        writer.uint32(/* id 10, wireType 2 =*/82).bytes(message.effectiveConfig);
                    if (message.meta != null && Object.hasOwnProperty.call(message, "meta"))
                        $root.cordum.agent.v1.JobMetadata.encode(message.meta, writer.uint32(/* id 11, wireType 2 =*/90).fork()).ldelim();
                    return writer;
                };

                /**
                 * Encodes the specified PolicyCheckRequest message, length delimited. Does not implicitly {@link cordum.agent.v1.PolicyCheckRequest.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {cordum.agent.v1.IPolicyCheckRequest} message PolicyCheckRequest message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                PolicyCheckRequest.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a PolicyCheckRequest message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.PolicyCheckRequest} PolicyCheckRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                PolicyCheckRequest.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.PolicyCheckRequest(), key, value;
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
                                message.budget = $root.cordum.agent.v1.Budget.decode(reader, reader.uint32());
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
                        case 10: {
                                message.effectiveConfig = reader.bytes();
                                break;
                            }
                        case 11: {
                                message.meta = $root.cordum.agent.v1.JobMetadata.decode(reader, reader.uint32());
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
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.PolicyCheckRequest} PolicyCheckRequest
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
                 * @memberof cordum.agent.v1.PolicyCheckRequest
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
                        var error = $root.cordum.agent.v1.Budget.verify(message.budget);
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
                    if (message.effectiveConfig != null && message.hasOwnProperty("effectiveConfig"))
                        if (!(message.effectiveConfig && typeof message.effectiveConfig.length === "number" || $util.isString(message.effectiveConfig)))
                            return "effectiveConfig: buffer expected";
                    if (message.meta != null && message.hasOwnProperty("meta")) {
                        var error = $root.cordum.agent.v1.JobMetadata.verify(message.meta);
                        if (error)
                            return "meta." + error;
                    }
                    return null;
                };

                /**
                 * Creates a PolicyCheckRequest message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.PolicyCheckRequest} PolicyCheckRequest
                 */
                PolicyCheckRequest.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.PolicyCheckRequest)
                        return object;
                    var message = new $root.cordum.agent.v1.PolicyCheckRequest();
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
                            throw TypeError(".cordum.agent.v1.PolicyCheckRequest.budget: object expected");
                        message.budget = $root.cordum.agent.v1.Budget.fromObject(object.budget);
                    }
                    if (object.principalId != null)
                        message.principalId = String(object.principalId);
                    if (object.labels) {
                        if (typeof object.labels !== "object")
                            throw TypeError(".cordum.agent.v1.PolicyCheckRequest.labels: object expected");
                        message.labels = {};
                        for (var keys = Object.keys(object.labels), i = 0; i < keys.length; ++i)
                            message.labels[keys[i]] = String(object.labels[keys[i]]);
                    }
                    if (object.memoryId != null)
                        message.memoryId = String(object.memoryId);
                    if (object.effectiveConfig != null)
                        if (typeof object.effectiveConfig === "string")
                            $util.base64.decode(object.effectiveConfig, message.effectiveConfig = $util.newBuffer($util.base64.length(object.effectiveConfig)), 0);
                        else if (object.effectiveConfig.length >= 0)
                            message.effectiveConfig = object.effectiveConfig;
                    if (object.meta != null) {
                        if (typeof object.meta !== "object")
                            throw TypeError(".cordum.agent.v1.PolicyCheckRequest.meta: object expected");
                        message.meta = $root.cordum.agent.v1.JobMetadata.fromObject(object.meta);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a PolicyCheckRequest message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {cordum.agent.v1.PolicyCheckRequest} message PolicyCheckRequest
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
                        if (options.bytes === String)
                            object.effectiveConfig = "";
                        else {
                            object.effectiveConfig = [];
                            if (options.bytes !== Array)
                                object.effectiveConfig = $util.newBuffer(object.effectiveConfig);
                        }
                        object.meta = null;
                    }
                    if (message.jobId != null && message.hasOwnProperty("jobId"))
                        object.jobId = message.jobId;
                    if (message.topic != null && message.hasOwnProperty("topic"))
                        object.topic = message.topic;
                    if (message.tenant != null && message.hasOwnProperty("tenant"))
                        object.tenant = message.tenant;
                    if (message.priority != null && message.hasOwnProperty("priority"))
                        object.priority = options.enums === String ? $root.cordum.agent.v1.JobPriority[message.priority] === undefined ? message.priority : $root.cordum.agent.v1.JobPriority[message.priority] : message.priority;
                    if (message.estimatedCost != null && message.hasOwnProperty("estimatedCost"))
                        object.estimatedCost = options.json && !isFinite(message.estimatedCost) ? String(message.estimatedCost) : message.estimatedCost;
                    if (message.budget != null && message.hasOwnProperty("budget"))
                        object.budget = $root.cordum.agent.v1.Budget.toObject(message.budget, options);
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
                    if (message.effectiveConfig != null && message.hasOwnProperty("effectiveConfig"))
                        object.effectiveConfig = options.bytes === String ? $util.base64.encode(message.effectiveConfig, 0, message.effectiveConfig.length) : options.bytes === Array ? Array.prototype.slice.call(message.effectiveConfig) : message.effectiveConfig;
                    if (message.meta != null && message.hasOwnProperty("meta"))
                        object.meta = $root.cordum.agent.v1.JobMetadata.toObject(message.meta, options);
                    return object;
                };

                /**
                 * Converts this PolicyCheckRequest to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                PolicyCheckRequest.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for PolicyCheckRequest
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.PolicyCheckRequest
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                PolicyCheckRequest.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.PolicyCheckRequest";
                };

                return PolicyCheckRequest;
            })();

            v1.BudgetConstraints = (function() {

                /**
                 * Properties of a BudgetConstraints.
                 * @memberof cordum.agent.v1
                 * @interface IBudgetConstraints
                 * @property {number|Long|null} [maxRuntimeMs] BudgetConstraints maxRuntimeMs
                 * @property {number|null} [maxRetries] BudgetConstraints maxRetries
                 * @property {number|Long|null} [maxArtifactBytes] BudgetConstraints maxArtifactBytes
                 * @property {number|null} [maxConcurrentJobs] BudgetConstraints maxConcurrentJobs
                 */

                /**
                 * Constructs a new BudgetConstraints.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a BudgetConstraints.
                 * @implements IBudgetConstraints
                 * @constructor
                 * @param {cordum.agent.v1.IBudgetConstraints=} [properties] Properties to set
                 */
                function BudgetConstraints(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * BudgetConstraints maxRuntimeMs.
                 * @member {number|Long} maxRuntimeMs
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @instance
                 */
                BudgetConstraints.prototype.maxRuntimeMs = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * BudgetConstraints maxRetries.
                 * @member {number} maxRetries
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @instance
                 */
                BudgetConstraints.prototype.maxRetries = 0;

                /**
                 * BudgetConstraints maxArtifactBytes.
                 * @member {number|Long} maxArtifactBytes
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @instance
                 */
                BudgetConstraints.prototype.maxArtifactBytes = $util.Long ? $util.Long.fromBits(0,0,false) : 0;

                /**
                 * BudgetConstraints maxConcurrentJobs.
                 * @member {number} maxConcurrentJobs
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @instance
                 */
                BudgetConstraints.prototype.maxConcurrentJobs = 0;

                /**
                 * Creates a new BudgetConstraints instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @static
                 * @param {cordum.agent.v1.IBudgetConstraints=} [properties] Properties to set
                 * @returns {cordum.agent.v1.BudgetConstraints} BudgetConstraints instance
                 */
                BudgetConstraints.create = function create(properties) {
                    return new BudgetConstraints(properties);
                };

                /**
                 * Encodes the specified BudgetConstraints message. Does not implicitly {@link cordum.agent.v1.BudgetConstraints.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @static
                 * @param {cordum.agent.v1.IBudgetConstraints} message BudgetConstraints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                BudgetConstraints.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.maxRuntimeMs != null && Object.hasOwnProperty.call(message, "maxRuntimeMs"))
                        writer.uint32(/* id 1, wireType 0 =*/8).int64(message.maxRuntimeMs);
                    if (message.maxRetries != null && Object.hasOwnProperty.call(message, "maxRetries"))
                        writer.uint32(/* id 2, wireType 0 =*/16).int32(message.maxRetries);
                    if (message.maxArtifactBytes != null && Object.hasOwnProperty.call(message, "maxArtifactBytes"))
                        writer.uint32(/* id 3, wireType 0 =*/24).int64(message.maxArtifactBytes);
                    if (message.maxConcurrentJobs != null && Object.hasOwnProperty.call(message, "maxConcurrentJobs"))
                        writer.uint32(/* id 4, wireType 0 =*/32).int32(message.maxConcurrentJobs);
                    return writer;
                };

                /**
                 * Encodes the specified BudgetConstraints message, length delimited. Does not implicitly {@link cordum.agent.v1.BudgetConstraints.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @static
                 * @param {cordum.agent.v1.IBudgetConstraints} message BudgetConstraints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                BudgetConstraints.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a BudgetConstraints message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.BudgetConstraints} BudgetConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                BudgetConstraints.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.BudgetConstraints();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.maxRuntimeMs = reader.int64();
                                break;
                            }
                        case 2: {
                                message.maxRetries = reader.int32();
                                break;
                            }
                        case 3: {
                                message.maxArtifactBytes = reader.int64();
                                break;
                            }
                        case 4: {
                                message.maxConcurrentJobs = reader.int32();
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
                 * Decodes a BudgetConstraints message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.BudgetConstraints} BudgetConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                BudgetConstraints.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a BudgetConstraints message.
                 * @function verify
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                BudgetConstraints.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.maxRuntimeMs != null && message.hasOwnProperty("maxRuntimeMs"))
                        if (!$util.isInteger(message.maxRuntimeMs) && !(message.maxRuntimeMs && $util.isInteger(message.maxRuntimeMs.low) && $util.isInteger(message.maxRuntimeMs.high)))
                            return "maxRuntimeMs: integer|Long expected";
                    if (message.maxRetries != null && message.hasOwnProperty("maxRetries"))
                        if (!$util.isInteger(message.maxRetries))
                            return "maxRetries: integer expected";
                    if (message.maxArtifactBytes != null && message.hasOwnProperty("maxArtifactBytes"))
                        if (!$util.isInteger(message.maxArtifactBytes) && !(message.maxArtifactBytes && $util.isInteger(message.maxArtifactBytes.low) && $util.isInteger(message.maxArtifactBytes.high)))
                            return "maxArtifactBytes: integer|Long expected";
                    if (message.maxConcurrentJobs != null && message.hasOwnProperty("maxConcurrentJobs"))
                        if (!$util.isInteger(message.maxConcurrentJobs))
                            return "maxConcurrentJobs: integer expected";
                    return null;
                };

                /**
                 * Creates a BudgetConstraints message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.BudgetConstraints} BudgetConstraints
                 */
                BudgetConstraints.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.BudgetConstraints)
                        return object;
                    var message = new $root.cordum.agent.v1.BudgetConstraints();
                    if (object.maxRuntimeMs != null)
                        if ($util.Long)
                            (message.maxRuntimeMs = $util.Long.fromValue(object.maxRuntimeMs)).unsigned = false;
                        else if (typeof object.maxRuntimeMs === "string")
                            message.maxRuntimeMs = parseInt(object.maxRuntimeMs, 10);
                        else if (typeof object.maxRuntimeMs === "number")
                            message.maxRuntimeMs = object.maxRuntimeMs;
                        else if (typeof object.maxRuntimeMs === "object")
                            message.maxRuntimeMs = new $util.LongBits(object.maxRuntimeMs.low >>> 0, object.maxRuntimeMs.high >>> 0).toNumber();
                    if (object.maxRetries != null)
                        message.maxRetries = object.maxRetries | 0;
                    if (object.maxArtifactBytes != null)
                        if ($util.Long)
                            (message.maxArtifactBytes = $util.Long.fromValue(object.maxArtifactBytes)).unsigned = false;
                        else if (typeof object.maxArtifactBytes === "string")
                            message.maxArtifactBytes = parseInt(object.maxArtifactBytes, 10);
                        else if (typeof object.maxArtifactBytes === "number")
                            message.maxArtifactBytes = object.maxArtifactBytes;
                        else if (typeof object.maxArtifactBytes === "object")
                            message.maxArtifactBytes = new $util.LongBits(object.maxArtifactBytes.low >>> 0, object.maxArtifactBytes.high >>> 0).toNumber();
                    if (object.maxConcurrentJobs != null)
                        message.maxConcurrentJobs = object.maxConcurrentJobs | 0;
                    return message;
                };

                /**
                 * Creates a plain object from a BudgetConstraints message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @static
                 * @param {cordum.agent.v1.BudgetConstraints} message BudgetConstraints
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                BudgetConstraints.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.defaults) {
                        if ($util.Long) {
                            var long = new $util.Long(0, 0, false);
                            object.maxRuntimeMs = options.longs === String ? long.toString() : options.longs === Number ? long.toNumber() : long;
                        } else
                            object.maxRuntimeMs = options.longs === String ? "0" : 0;
                        object.maxRetries = 0;
                        if ($util.Long) {
                            var long = new $util.Long(0, 0, false);
                            object.maxArtifactBytes = options.longs === String ? long.toString() : options.longs === Number ? long.toNumber() : long;
                        } else
                            object.maxArtifactBytes = options.longs === String ? "0" : 0;
                        object.maxConcurrentJobs = 0;
                    }
                    if (message.maxRuntimeMs != null && message.hasOwnProperty("maxRuntimeMs"))
                        if (typeof message.maxRuntimeMs === "number")
                            object.maxRuntimeMs = options.longs === String ? String(message.maxRuntimeMs) : message.maxRuntimeMs;
                        else
                            object.maxRuntimeMs = options.longs === String ? $util.Long.prototype.toString.call(message.maxRuntimeMs) : options.longs === Number ? new $util.LongBits(message.maxRuntimeMs.low >>> 0, message.maxRuntimeMs.high >>> 0).toNumber() : message.maxRuntimeMs;
                    if (message.maxRetries != null && message.hasOwnProperty("maxRetries"))
                        object.maxRetries = message.maxRetries;
                    if (message.maxArtifactBytes != null && message.hasOwnProperty("maxArtifactBytes"))
                        if (typeof message.maxArtifactBytes === "number")
                            object.maxArtifactBytes = options.longs === String ? String(message.maxArtifactBytes) : message.maxArtifactBytes;
                        else
                            object.maxArtifactBytes = options.longs === String ? $util.Long.prototype.toString.call(message.maxArtifactBytes) : options.longs === Number ? new $util.LongBits(message.maxArtifactBytes.low >>> 0, message.maxArtifactBytes.high >>> 0).toNumber() : message.maxArtifactBytes;
                    if (message.maxConcurrentJobs != null && message.hasOwnProperty("maxConcurrentJobs"))
                        object.maxConcurrentJobs = message.maxConcurrentJobs;
                    return object;
                };

                /**
                 * Converts this BudgetConstraints to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                BudgetConstraints.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for BudgetConstraints
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.BudgetConstraints
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                BudgetConstraints.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.BudgetConstraints";
                };

                return BudgetConstraints;
            })();

            v1.SandboxProfile = (function() {

                /**
                 * Properties of a SandboxProfile.
                 * @memberof cordum.agent.v1
                 * @interface ISandboxProfile
                 * @property {boolean|null} [isolated] SandboxProfile isolated
                 * @property {Array.<string>|null} [networkAllowlist] SandboxProfile networkAllowlist
                 * @property {Array.<string>|null} [fsReadOnly] SandboxProfile fsReadOnly
                 * @property {Array.<string>|null} [fsReadWrite] SandboxProfile fsReadWrite
                 */

                /**
                 * Constructs a new SandboxProfile.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a SandboxProfile.
                 * @implements ISandboxProfile
                 * @constructor
                 * @param {cordum.agent.v1.ISandboxProfile=} [properties] Properties to set
                 */
                function SandboxProfile(properties) {
                    this.networkAllowlist = [];
                    this.fsReadOnly = [];
                    this.fsReadWrite = [];
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * SandboxProfile isolated.
                 * @member {boolean} isolated
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @instance
                 */
                SandboxProfile.prototype.isolated = false;

                /**
                 * SandboxProfile networkAllowlist.
                 * @member {Array.<string>} networkAllowlist
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @instance
                 */
                SandboxProfile.prototype.networkAllowlist = $util.emptyArray;

                /**
                 * SandboxProfile fsReadOnly.
                 * @member {Array.<string>} fsReadOnly
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @instance
                 */
                SandboxProfile.prototype.fsReadOnly = $util.emptyArray;

                /**
                 * SandboxProfile fsReadWrite.
                 * @member {Array.<string>} fsReadWrite
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @instance
                 */
                SandboxProfile.prototype.fsReadWrite = $util.emptyArray;

                /**
                 * Creates a new SandboxProfile instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @static
                 * @param {cordum.agent.v1.ISandboxProfile=} [properties] Properties to set
                 * @returns {cordum.agent.v1.SandboxProfile} SandboxProfile instance
                 */
                SandboxProfile.create = function create(properties) {
                    return new SandboxProfile(properties);
                };

                /**
                 * Encodes the specified SandboxProfile message. Does not implicitly {@link cordum.agent.v1.SandboxProfile.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @static
                 * @param {cordum.agent.v1.ISandboxProfile} message SandboxProfile message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                SandboxProfile.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.isolated != null && Object.hasOwnProperty.call(message, "isolated"))
                        writer.uint32(/* id 1, wireType 0 =*/8).bool(message.isolated);
                    if (message.networkAllowlist != null && message.networkAllowlist.length)
                        for (var i = 0; i < message.networkAllowlist.length; ++i)
                            writer.uint32(/* id 2, wireType 2 =*/18).string(message.networkAllowlist[i]);
                    if (message.fsReadOnly != null && message.fsReadOnly.length)
                        for (var i = 0; i < message.fsReadOnly.length; ++i)
                            writer.uint32(/* id 3, wireType 2 =*/26).string(message.fsReadOnly[i]);
                    if (message.fsReadWrite != null && message.fsReadWrite.length)
                        for (var i = 0; i < message.fsReadWrite.length; ++i)
                            writer.uint32(/* id 4, wireType 2 =*/34).string(message.fsReadWrite[i]);
                    return writer;
                };

                /**
                 * Encodes the specified SandboxProfile message, length delimited. Does not implicitly {@link cordum.agent.v1.SandboxProfile.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @static
                 * @param {cordum.agent.v1.ISandboxProfile} message SandboxProfile message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                SandboxProfile.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a SandboxProfile message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.SandboxProfile} SandboxProfile
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                SandboxProfile.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.SandboxProfile();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.isolated = reader.bool();
                                break;
                            }
                        case 2: {
                                if (!(message.networkAllowlist && message.networkAllowlist.length))
                                    message.networkAllowlist = [];
                                message.networkAllowlist.push(reader.string());
                                break;
                            }
                        case 3: {
                                if (!(message.fsReadOnly && message.fsReadOnly.length))
                                    message.fsReadOnly = [];
                                message.fsReadOnly.push(reader.string());
                                break;
                            }
                        case 4: {
                                if (!(message.fsReadWrite && message.fsReadWrite.length))
                                    message.fsReadWrite = [];
                                message.fsReadWrite.push(reader.string());
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
                 * Decodes a SandboxProfile message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.SandboxProfile} SandboxProfile
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                SandboxProfile.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a SandboxProfile message.
                 * @function verify
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                SandboxProfile.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.isolated != null && message.hasOwnProperty("isolated"))
                        if (typeof message.isolated !== "boolean")
                            return "isolated: boolean expected";
                    if (message.networkAllowlist != null && message.hasOwnProperty("networkAllowlist")) {
                        if (!Array.isArray(message.networkAllowlist))
                            return "networkAllowlist: array expected";
                        for (var i = 0; i < message.networkAllowlist.length; ++i)
                            if (!$util.isString(message.networkAllowlist[i]))
                                return "networkAllowlist: string[] expected";
                    }
                    if (message.fsReadOnly != null && message.hasOwnProperty("fsReadOnly")) {
                        if (!Array.isArray(message.fsReadOnly))
                            return "fsReadOnly: array expected";
                        for (var i = 0; i < message.fsReadOnly.length; ++i)
                            if (!$util.isString(message.fsReadOnly[i]))
                                return "fsReadOnly: string[] expected";
                    }
                    if (message.fsReadWrite != null && message.hasOwnProperty("fsReadWrite")) {
                        if (!Array.isArray(message.fsReadWrite))
                            return "fsReadWrite: array expected";
                        for (var i = 0; i < message.fsReadWrite.length; ++i)
                            if (!$util.isString(message.fsReadWrite[i]))
                                return "fsReadWrite: string[] expected";
                    }
                    return null;
                };

                /**
                 * Creates a SandboxProfile message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.SandboxProfile} SandboxProfile
                 */
                SandboxProfile.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.SandboxProfile)
                        return object;
                    var message = new $root.cordum.agent.v1.SandboxProfile();
                    if (object.isolated != null)
                        message.isolated = Boolean(object.isolated);
                    if (object.networkAllowlist) {
                        if (!Array.isArray(object.networkAllowlist))
                            throw TypeError(".cordum.agent.v1.SandboxProfile.networkAllowlist: array expected");
                        message.networkAllowlist = [];
                        for (var i = 0; i < object.networkAllowlist.length; ++i)
                            message.networkAllowlist[i] = String(object.networkAllowlist[i]);
                    }
                    if (object.fsReadOnly) {
                        if (!Array.isArray(object.fsReadOnly))
                            throw TypeError(".cordum.agent.v1.SandboxProfile.fsReadOnly: array expected");
                        message.fsReadOnly = [];
                        for (var i = 0; i < object.fsReadOnly.length; ++i)
                            message.fsReadOnly[i] = String(object.fsReadOnly[i]);
                    }
                    if (object.fsReadWrite) {
                        if (!Array.isArray(object.fsReadWrite))
                            throw TypeError(".cordum.agent.v1.SandboxProfile.fsReadWrite: array expected");
                        message.fsReadWrite = [];
                        for (var i = 0; i < object.fsReadWrite.length; ++i)
                            message.fsReadWrite[i] = String(object.fsReadWrite[i]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a SandboxProfile message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @static
                 * @param {cordum.agent.v1.SandboxProfile} message SandboxProfile
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                SandboxProfile.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults) {
                        object.networkAllowlist = [];
                        object.fsReadOnly = [];
                        object.fsReadWrite = [];
                    }
                    if (options.defaults)
                        object.isolated = false;
                    if (message.isolated != null && message.hasOwnProperty("isolated"))
                        object.isolated = message.isolated;
                    if (message.networkAllowlist && message.networkAllowlist.length) {
                        object.networkAllowlist = [];
                        for (var j = 0; j < message.networkAllowlist.length; ++j)
                            object.networkAllowlist[j] = message.networkAllowlist[j];
                    }
                    if (message.fsReadOnly && message.fsReadOnly.length) {
                        object.fsReadOnly = [];
                        for (var j = 0; j < message.fsReadOnly.length; ++j)
                            object.fsReadOnly[j] = message.fsReadOnly[j];
                    }
                    if (message.fsReadWrite && message.fsReadWrite.length) {
                        object.fsReadWrite = [];
                        for (var j = 0; j < message.fsReadWrite.length; ++j)
                            object.fsReadWrite[j] = message.fsReadWrite[j];
                    }
                    return object;
                };

                /**
                 * Converts this SandboxProfile to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                SandboxProfile.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for SandboxProfile
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.SandboxProfile
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                SandboxProfile.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.SandboxProfile";
                };

                return SandboxProfile;
            })();

            v1.ToolchainConstraints = (function() {

                /**
                 * Properties of a ToolchainConstraints.
                 * @memberof cordum.agent.v1
                 * @interface IToolchainConstraints
                 * @property {Array.<string>|null} [allowedTools] ToolchainConstraints allowedTools
                 * @property {Array.<string>|null} [allowedCommands] ToolchainConstraints allowedCommands
                 */

                /**
                 * Constructs a new ToolchainConstraints.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a ToolchainConstraints.
                 * @implements IToolchainConstraints
                 * @constructor
                 * @param {cordum.agent.v1.IToolchainConstraints=} [properties] Properties to set
                 */
                function ToolchainConstraints(properties) {
                    this.allowedTools = [];
                    this.allowedCommands = [];
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * ToolchainConstraints allowedTools.
                 * @member {Array.<string>} allowedTools
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @instance
                 */
                ToolchainConstraints.prototype.allowedTools = $util.emptyArray;

                /**
                 * ToolchainConstraints allowedCommands.
                 * @member {Array.<string>} allowedCommands
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @instance
                 */
                ToolchainConstraints.prototype.allowedCommands = $util.emptyArray;

                /**
                 * Creates a new ToolchainConstraints instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @static
                 * @param {cordum.agent.v1.IToolchainConstraints=} [properties] Properties to set
                 * @returns {cordum.agent.v1.ToolchainConstraints} ToolchainConstraints instance
                 */
                ToolchainConstraints.create = function create(properties) {
                    return new ToolchainConstraints(properties);
                };

                /**
                 * Encodes the specified ToolchainConstraints message. Does not implicitly {@link cordum.agent.v1.ToolchainConstraints.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @static
                 * @param {cordum.agent.v1.IToolchainConstraints} message ToolchainConstraints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ToolchainConstraints.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.allowedTools != null && message.allowedTools.length)
                        for (var i = 0; i < message.allowedTools.length; ++i)
                            writer.uint32(/* id 1, wireType 2 =*/10).string(message.allowedTools[i]);
                    if (message.allowedCommands != null && message.allowedCommands.length)
                        for (var i = 0; i < message.allowedCommands.length; ++i)
                            writer.uint32(/* id 2, wireType 2 =*/18).string(message.allowedCommands[i]);
                    return writer;
                };

                /**
                 * Encodes the specified ToolchainConstraints message, length delimited. Does not implicitly {@link cordum.agent.v1.ToolchainConstraints.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @static
                 * @param {cordum.agent.v1.IToolchainConstraints} message ToolchainConstraints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ToolchainConstraints.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a ToolchainConstraints message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.ToolchainConstraints} ToolchainConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ToolchainConstraints.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.ToolchainConstraints();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                if (!(message.allowedTools && message.allowedTools.length))
                                    message.allowedTools = [];
                                message.allowedTools.push(reader.string());
                                break;
                            }
                        case 2: {
                                if (!(message.allowedCommands && message.allowedCommands.length))
                                    message.allowedCommands = [];
                                message.allowedCommands.push(reader.string());
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
                 * Decodes a ToolchainConstraints message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.ToolchainConstraints} ToolchainConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ToolchainConstraints.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a ToolchainConstraints message.
                 * @function verify
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                ToolchainConstraints.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.allowedTools != null && message.hasOwnProperty("allowedTools")) {
                        if (!Array.isArray(message.allowedTools))
                            return "allowedTools: array expected";
                        for (var i = 0; i < message.allowedTools.length; ++i)
                            if (!$util.isString(message.allowedTools[i]))
                                return "allowedTools: string[] expected";
                    }
                    if (message.allowedCommands != null && message.hasOwnProperty("allowedCommands")) {
                        if (!Array.isArray(message.allowedCommands))
                            return "allowedCommands: array expected";
                        for (var i = 0; i < message.allowedCommands.length; ++i)
                            if (!$util.isString(message.allowedCommands[i]))
                                return "allowedCommands: string[] expected";
                    }
                    return null;
                };

                /**
                 * Creates a ToolchainConstraints message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.ToolchainConstraints} ToolchainConstraints
                 */
                ToolchainConstraints.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.ToolchainConstraints)
                        return object;
                    var message = new $root.cordum.agent.v1.ToolchainConstraints();
                    if (object.allowedTools) {
                        if (!Array.isArray(object.allowedTools))
                            throw TypeError(".cordum.agent.v1.ToolchainConstraints.allowedTools: array expected");
                        message.allowedTools = [];
                        for (var i = 0; i < object.allowedTools.length; ++i)
                            message.allowedTools[i] = String(object.allowedTools[i]);
                    }
                    if (object.allowedCommands) {
                        if (!Array.isArray(object.allowedCommands))
                            throw TypeError(".cordum.agent.v1.ToolchainConstraints.allowedCommands: array expected");
                        message.allowedCommands = [];
                        for (var i = 0; i < object.allowedCommands.length; ++i)
                            message.allowedCommands[i] = String(object.allowedCommands[i]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a ToolchainConstraints message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @static
                 * @param {cordum.agent.v1.ToolchainConstraints} message ToolchainConstraints
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                ToolchainConstraints.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults) {
                        object.allowedTools = [];
                        object.allowedCommands = [];
                    }
                    if (message.allowedTools && message.allowedTools.length) {
                        object.allowedTools = [];
                        for (var j = 0; j < message.allowedTools.length; ++j)
                            object.allowedTools[j] = message.allowedTools[j];
                    }
                    if (message.allowedCommands && message.allowedCommands.length) {
                        object.allowedCommands = [];
                        for (var j = 0; j < message.allowedCommands.length; ++j)
                            object.allowedCommands[j] = message.allowedCommands[j];
                    }
                    return object;
                };

                /**
                 * Converts this ToolchainConstraints to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                ToolchainConstraints.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for ToolchainConstraints
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.ToolchainConstraints
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                ToolchainConstraints.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.ToolchainConstraints";
                };

                return ToolchainConstraints;
            })();

            v1.DiffConstraints = (function() {

                /**
                 * Properties of a DiffConstraints.
                 * @memberof cordum.agent.v1
                 * @interface IDiffConstraints
                 * @property {number|null} [maxFiles] DiffConstraints maxFiles
                 * @property {number|null} [maxLines] DiffConstraints maxLines
                 * @property {Array.<string>|null} [denyPathGlobs] DiffConstraints denyPathGlobs
                 */

                /**
                 * Constructs a new DiffConstraints.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a DiffConstraints.
                 * @implements IDiffConstraints
                 * @constructor
                 * @param {cordum.agent.v1.IDiffConstraints=} [properties] Properties to set
                 */
                function DiffConstraints(properties) {
                    this.denyPathGlobs = [];
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * DiffConstraints maxFiles.
                 * @member {number} maxFiles
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @instance
                 */
                DiffConstraints.prototype.maxFiles = 0;

                /**
                 * DiffConstraints maxLines.
                 * @member {number} maxLines
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @instance
                 */
                DiffConstraints.prototype.maxLines = 0;

                /**
                 * DiffConstraints denyPathGlobs.
                 * @member {Array.<string>} denyPathGlobs
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @instance
                 */
                DiffConstraints.prototype.denyPathGlobs = $util.emptyArray;

                /**
                 * Creates a new DiffConstraints instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @static
                 * @param {cordum.agent.v1.IDiffConstraints=} [properties] Properties to set
                 * @returns {cordum.agent.v1.DiffConstraints} DiffConstraints instance
                 */
                DiffConstraints.create = function create(properties) {
                    return new DiffConstraints(properties);
                };

                /**
                 * Encodes the specified DiffConstraints message. Does not implicitly {@link cordum.agent.v1.DiffConstraints.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @static
                 * @param {cordum.agent.v1.IDiffConstraints} message DiffConstraints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                DiffConstraints.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.maxFiles != null && Object.hasOwnProperty.call(message, "maxFiles"))
                        writer.uint32(/* id 1, wireType 0 =*/8).int32(message.maxFiles);
                    if (message.maxLines != null && Object.hasOwnProperty.call(message, "maxLines"))
                        writer.uint32(/* id 2, wireType 0 =*/16).int32(message.maxLines);
                    if (message.denyPathGlobs != null && message.denyPathGlobs.length)
                        for (var i = 0; i < message.denyPathGlobs.length; ++i)
                            writer.uint32(/* id 3, wireType 2 =*/26).string(message.denyPathGlobs[i]);
                    return writer;
                };

                /**
                 * Encodes the specified DiffConstraints message, length delimited. Does not implicitly {@link cordum.agent.v1.DiffConstraints.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @static
                 * @param {cordum.agent.v1.IDiffConstraints} message DiffConstraints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                DiffConstraints.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a DiffConstraints message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.DiffConstraints} DiffConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                DiffConstraints.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.DiffConstraints();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.maxFiles = reader.int32();
                                break;
                            }
                        case 2: {
                                message.maxLines = reader.int32();
                                break;
                            }
                        case 3: {
                                if (!(message.denyPathGlobs && message.denyPathGlobs.length))
                                    message.denyPathGlobs = [];
                                message.denyPathGlobs.push(reader.string());
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
                 * Decodes a DiffConstraints message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.DiffConstraints} DiffConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                DiffConstraints.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a DiffConstraints message.
                 * @function verify
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                DiffConstraints.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.maxFiles != null && message.hasOwnProperty("maxFiles"))
                        if (!$util.isInteger(message.maxFiles))
                            return "maxFiles: integer expected";
                    if (message.maxLines != null && message.hasOwnProperty("maxLines"))
                        if (!$util.isInteger(message.maxLines))
                            return "maxLines: integer expected";
                    if (message.denyPathGlobs != null && message.hasOwnProperty("denyPathGlobs")) {
                        if (!Array.isArray(message.denyPathGlobs))
                            return "denyPathGlobs: array expected";
                        for (var i = 0; i < message.denyPathGlobs.length; ++i)
                            if (!$util.isString(message.denyPathGlobs[i]))
                                return "denyPathGlobs: string[] expected";
                    }
                    return null;
                };

                /**
                 * Creates a DiffConstraints message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.DiffConstraints} DiffConstraints
                 */
                DiffConstraints.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.DiffConstraints)
                        return object;
                    var message = new $root.cordum.agent.v1.DiffConstraints();
                    if (object.maxFiles != null)
                        message.maxFiles = object.maxFiles | 0;
                    if (object.maxLines != null)
                        message.maxLines = object.maxLines | 0;
                    if (object.denyPathGlobs) {
                        if (!Array.isArray(object.denyPathGlobs))
                            throw TypeError(".cordum.agent.v1.DiffConstraints.denyPathGlobs: array expected");
                        message.denyPathGlobs = [];
                        for (var i = 0; i < object.denyPathGlobs.length; ++i)
                            message.denyPathGlobs[i] = String(object.denyPathGlobs[i]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a DiffConstraints message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @static
                 * @param {cordum.agent.v1.DiffConstraints} message DiffConstraints
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                DiffConstraints.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults)
                        object.denyPathGlobs = [];
                    if (options.defaults) {
                        object.maxFiles = 0;
                        object.maxLines = 0;
                    }
                    if (message.maxFiles != null && message.hasOwnProperty("maxFiles"))
                        object.maxFiles = message.maxFiles;
                    if (message.maxLines != null && message.hasOwnProperty("maxLines"))
                        object.maxLines = message.maxLines;
                    if (message.denyPathGlobs && message.denyPathGlobs.length) {
                        object.denyPathGlobs = [];
                        for (var j = 0; j < message.denyPathGlobs.length; ++j)
                            object.denyPathGlobs[j] = message.denyPathGlobs[j];
                    }
                    return object;
                };

                /**
                 * Converts this DiffConstraints to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                DiffConstraints.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for DiffConstraints
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.DiffConstraints
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                DiffConstraints.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.DiffConstraints";
                };

                return DiffConstraints;
            })();

            v1.PolicyConstraints = (function() {

                /**
                 * Properties of a PolicyConstraints.
                 * @memberof cordum.agent.v1
                 * @interface IPolicyConstraints
                 * @property {cordum.agent.v1.IBudgetConstraints|null} [budgets] PolicyConstraints budgets
                 * @property {cordum.agent.v1.ISandboxProfile|null} [sandbox] PolicyConstraints sandbox
                 * @property {cordum.agent.v1.IToolchainConstraints|null} [toolchain] PolicyConstraints toolchain
                 * @property {cordum.agent.v1.IDiffConstraints|null} [diff] PolicyConstraints diff
                 * @property {string|null} [redactionLevel] PolicyConstraints redactionLevel
                 */

                /**
                 * Constructs a new PolicyConstraints.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a PolicyConstraints.
                 * @implements IPolicyConstraints
                 * @constructor
                 * @param {cordum.agent.v1.IPolicyConstraints=} [properties] Properties to set
                 */
                function PolicyConstraints(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * PolicyConstraints budgets.
                 * @member {cordum.agent.v1.IBudgetConstraints|null|undefined} budgets
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @instance
                 */
                PolicyConstraints.prototype.budgets = null;

                /**
                 * PolicyConstraints sandbox.
                 * @member {cordum.agent.v1.ISandboxProfile|null|undefined} sandbox
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @instance
                 */
                PolicyConstraints.prototype.sandbox = null;

                /**
                 * PolicyConstraints toolchain.
                 * @member {cordum.agent.v1.IToolchainConstraints|null|undefined} toolchain
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @instance
                 */
                PolicyConstraints.prototype.toolchain = null;

                /**
                 * PolicyConstraints diff.
                 * @member {cordum.agent.v1.IDiffConstraints|null|undefined} diff
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @instance
                 */
                PolicyConstraints.prototype.diff = null;

                /**
                 * PolicyConstraints redactionLevel.
                 * @member {string} redactionLevel
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @instance
                 */
                PolicyConstraints.prototype.redactionLevel = "";

                /**
                 * Creates a new PolicyConstraints instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @static
                 * @param {cordum.agent.v1.IPolicyConstraints=} [properties] Properties to set
                 * @returns {cordum.agent.v1.PolicyConstraints} PolicyConstraints instance
                 */
                PolicyConstraints.create = function create(properties) {
                    return new PolicyConstraints(properties);
                };

                /**
                 * Encodes the specified PolicyConstraints message. Does not implicitly {@link cordum.agent.v1.PolicyConstraints.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @static
                 * @param {cordum.agent.v1.IPolicyConstraints} message PolicyConstraints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                PolicyConstraints.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.budgets != null && Object.hasOwnProperty.call(message, "budgets"))
                        $root.cordum.agent.v1.BudgetConstraints.encode(message.budgets, writer.uint32(/* id 1, wireType 2 =*/10).fork()).ldelim();
                    if (message.sandbox != null && Object.hasOwnProperty.call(message, "sandbox"))
                        $root.cordum.agent.v1.SandboxProfile.encode(message.sandbox, writer.uint32(/* id 2, wireType 2 =*/18).fork()).ldelim();
                    if (message.toolchain != null && Object.hasOwnProperty.call(message, "toolchain"))
                        $root.cordum.agent.v1.ToolchainConstraints.encode(message.toolchain, writer.uint32(/* id 3, wireType 2 =*/26).fork()).ldelim();
                    if (message.diff != null && Object.hasOwnProperty.call(message, "diff"))
                        $root.cordum.agent.v1.DiffConstraints.encode(message.diff, writer.uint32(/* id 4, wireType 2 =*/34).fork()).ldelim();
                    if (message.redactionLevel != null && Object.hasOwnProperty.call(message, "redactionLevel"))
                        writer.uint32(/* id 5, wireType 2 =*/42).string(message.redactionLevel);
                    return writer;
                };

                /**
                 * Encodes the specified PolicyConstraints message, length delimited. Does not implicitly {@link cordum.agent.v1.PolicyConstraints.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @static
                 * @param {cordum.agent.v1.IPolicyConstraints} message PolicyConstraints message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                PolicyConstraints.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a PolicyConstraints message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.PolicyConstraints} PolicyConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                PolicyConstraints.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.PolicyConstraints();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                message.budgets = $root.cordum.agent.v1.BudgetConstraints.decode(reader, reader.uint32());
                                break;
                            }
                        case 2: {
                                message.sandbox = $root.cordum.agent.v1.SandboxProfile.decode(reader, reader.uint32());
                                break;
                            }
                        case 3: {
                                message.toolchain = $root.cordum.agent.v1.ToolchainConstraints.decode(reader, reader.uint32());
                                break;
                            }
                        case 4: {
                                message.diff = $root.cordum.agent.v1.DiffConstraints.decode(reader, reader.uint32());
                                break;
                            }
                        case 5: {
                                message.redactionLevel = reader.string();
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
                 * Decodes a PolicyConstraints message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.PolicyConstraints} PolicyConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                PolicyConstraints.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a PolicyConstraints message.
                 * @function verify
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                PolicyConstraints.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.budgets != null && message.hasOwnProperty("budgets")) {
                        var error = $root.cordum.agent.v1.BudgetConstraints.verify(message.budgets);
                        if (error)
                            return "budgets." + error;
                    }
                    if (message.sandbox != null && message.hasOwnProperty("sandbox")) {
                        var error = $root.cordum.agent.v1.SandboxProfile.verify(message.sandbox);
                        if (error)
                            return "sandbox." + error;
                    }
                    if (message.toolchain != null && message.hasOwnProperty("toolchain")) {
                        var error = $root.cordum.agent.v1.ToolchainConstraints.verify(message.toolchain);
                        if (error)
                            return "toolchain." + error;
                    }
                    if (message.diff != null && message.hasOwnProperty("diff")) {
                        var error = $root.cordum.agent.v1.DiffConstraints.verify(message.diff);
                        if (error)
                            return "diff." + error;
                    }
                    if (message.redactionLevel != null && message.hasOwnProperty("redactionLevel"))
                        if (!$util.isString(message.redactionLevel))
                            return "redactionLevel: string expected";
                    return null;
                };

                /**
                 * Creates a PolicyConstraints message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.PolicyConstraints} PolicyConstraints
                 */
                PolicyConstraints.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.PolicyConstraints)
                        return object;
                    var message = new $root.cordum.agent.v1.PolicyConstraints();
                    if (object.budgets != null) {
                        if (typeof object.budgets !== "object")
                            throw TypeError(".cordum.agent.v1.PolicyConstraints.budgets: object expected");
                        message.budgets = $root.cordum.agent.v1.BudgetConstraints.fromObject(object.budgets);
                    }
                    if (object.sandbox != null) {
                        if (typeof object.sandbox !== "object")
                            throw TypeError(".cordum.agent.v1.PolicyConstraints.sandbox: object expected");
                        message.sandbox = $root.cordum.agent.v1.SandboxProfile.fromObject(object.sandbox);
                    }
                    if (object.toolchain != null) {
                        if (typeof object.toolchain !== "object")
                            throw TypeError(".cordum.agent.v1.PolicyConstraints.toolchain: object expected");
                        message.toolchain = $root.cordum.agent.v1.ToolchainConstraints.fromObject(object.toolchain);
                    }
                    if (object.diff != null) {
                        if (typeof object.diff !== "object")
                            throw TypeError(".cordum.agent.v1.PolicyConstraints.diff: object expected");
                        message.diff = $root.cordum.agent.v1.DiffConstraints.fromObject(object.diff);
                    }
                    if (object.redactionLevel != null)
                        message.redactionLevel = String(object.redactionLevel);
                    return message;
                };

                /**
                 * Creates a plain object from a PolicyConstraints message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @static
                 * @param {cordum.agent.v1.PolicyConstraints} message PolicyConstraints
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                PolicyConstraints.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.defaults) {
                        object.budgets = null;
                        object.sandbox = null;
                        object.toolchain = null;
                        object.diff = null;
                        object.redactionLevel = "";
                    }
                    if (message.budgets != null && message.hasOwnProperty("budgets"))
                        object.budgets = $root.cordum.agent.v1.BudgetConstraints.toObject(message.budgets, options);
                    if (message.sandbox != null && message.hasOwnProperty("sandbox"))
                        object.sandbox = $root.cordum.agent.v1.SandboxProfile.toObject(message.sandbox, options);
                    if (message.toolchain != null && message.hasOwnProperty("toolchain"))
                        object.toolchain = $root.cordum.agent.v1.ToolchainConstraints.toObject(message.toolchain, options);
                    if (message.diff != null && message.hasOwnProperty("diff"))
                        object.diff = $root.cordum.agent.v1.DiffConstraints.toObject(message.diff, options);
                    if (message.redactionLevel != null && message.hasOwnProperty("redactionLevel"))
                        object.redactionLevel = message.redactionLevel;
                    return object;
                };

                /**
                 * Converts this PolicyConstraints to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                PolicyConstraints.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for PolicyConstraints
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.PolicyConstraints
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                PolicyConstraints.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.PolicyConstraints";
                };

                return PolicyConstraints;
            })();

            v1.PolicyCheckResponse = (function() {

                /**
                 * Properties of a PolicyCheckResponse.
                 * @memberof cordum.agent.v1
                 * @interface IPolicyCheckResponse
                 * @property {cordum.agent.v1.DecisionType|null} [decision] PolicyCheckResponse decision
                 * @property {string|null} [reason] PolicyCheckResponse reason
                 * @property {string|null} [redactedContextPtr] PolicyCheckResponse redactedContextPtr
                 * @property {string|null} [policySnapshot] PolicyCheckResponse policySnapshot
                 * @property {string|null} [ruleId] PolicyCheckResponse ruleId
                 * @property {cordum.agent.v1.IPolicyConstraints|null} [constraints] PolicyCheckResponse constraints
                 * @property {boolean|null} [approvalRequired] PolicyCheckResponse approvalRequired
                 * @property {string|null} [approvalRef] PolicyCheckResponse approvalRef
                 */

                /**
                 * Constructs a new PolicyCheckResponse.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a PolicyCheckResponse.
                 * @implements IPolicyCheckResponse
                 * @constructor
                 * @param {cordum.agent.v1.IPolicyCheckResponse=} [properties] Properties to set
                 */
                function PolicyCheckResponse(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * PolicyCheckResponse decision.
                 * @member {cordum.agent.v1.DecisionType} decision
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.decision = 0;

                /**
                 * PolicyCheckResponse reason.
                 * @member {string} reason
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.reason = "";

                /**
                 * PolicyCheckResponse redactedContextPtr.
                 * @member {string} redactedContextPtr
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.redactedContextPtr = "";

                /**
                 * PolicyCheckResponse policySnapshot.
                 * @member {string} policySnapshot
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.policySnapshot = "";

                /**
                 * PolicyCheckResponse ruleId.
                 * @member {string} ruleId
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.ruleId = "";

                /**
                 * PolicyCheckResponse constraints.
                 * @member {cordum.agent.v1.IPolicyConstraints|null|undefined} constraints
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.constraints = null;

                /**
                 * PolicyCheckResponse approvalRequired.
                 * @member {boolean} approvalRequired
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.approvalRequired = false;

                /**
                 * PolicyCheckResponse approvalRef.
                 * @member {string} approvalRef
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @instance
                 */
                PolicyCheckResponse.prototype.approvalRef = "";

                /**
                 * Creates a new PolicyCheckResponse instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {cordum.agent.v1.IPolicyCheckResponse=} [properties] Properties to set
                 * @returns {cordum.agent.v1.PolicyCheckResponse} PolicyCheckResponse instance
                 */
                PolicyCheckResponse.create = function create(properties) {
                    return new PolicyCheckResponse(properties);
                };

                /**
                 * Encodes the specified PolicyCheckResponse message. Does not implicitly {@link cordum.agent.v1.PolicyCheckResponse.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {cordum.agent.v1.IPolicyCheckResponse} message PolicyCheckResponse message or plain object to encode
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
                    if (message.policySnapshot != null && Object.hasOwnProperty.call(message, "policySnapshot"))
                        writer.uint32(/* id 4, wireType 2 =*/34).string(message.policySnapshot);
                    if (message.ruleId != null && Object.hasOwnProperty.call(message, "ruleId"))
                        writer.uint32(/* id 5, wireType 2 =*/42).string(message.ruleId);
                    if (message.constraints != null && Object.hasOwnProperty.call(message, "constraints"))
                        $root.cordum.agent.v1.PolicyConstraints.encode(message.constraints, writer.uint32(/* id 6, wireType 2 =*/50).fork()).ldelim();
                    if (message.approvalRequired != null && Object.hasOwnProperty.call(message, "approvalRequired"))
                        writer.uint32(/* id 7, wireType 0 =*/56).bool(message.approvalRequired);
                    if (message.approvalRef != null && Object.hasOwnProperty.call(message, "approvalRef"))
                        writer.uint32(/* id 8, wireType 2 =*/66).string(message.approvalRef);
                    return writer;
                };

                /**
                 * Encodes the specified PolicyCheckResponse message, length delimited. Does not implicitly {@link cordum.agent.v1.PolicyCheckResponse.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {cordum.agent.v1.IPolicyCheckResponse} message PolicyCheckResponse message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                PolicyCheckResponse.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a PolicyCheckResponse message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.PolicyCheckResponse} PolicyCheckResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                PolicyCheckResponse.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.PolicyCheckResponse();
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
                        case 4: {
                                message.policySnapshot = reader.string();
                                break;
                            }
                        case 5: {
                                message.ruleId = reader.string();
                                break;
                            }
                        case 6: {
                                message.constraints = $root.cordum.agent.v1.PolicyConstraints.decode(reader, reader.uint32());
                                break;
                            }
                        case 7: {
                                message.approvalRequired = reader.bool();
                                break;
                            }
                        case 8: {
                                message.approvalRef = reader.string();
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
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.PolicyCheckResponse} PolicyCheckResponse
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
                 * @memberof cordum.agent.v1.PolicyCheckResponse
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
                        case 5:
                            break;
                        }
                    if (message.reason != null && message.hasOwnProperty("reason"))
                        if (!$util.isString(message.reason))
                            return "reason: string expected";
                    if (message.redactedContextPtr != null && message.hasOwnProperty("redactedContextPtr"))
                        if (!$util.isString(message.redactedContextPtr))
                            return "redactedContextPtr: string expected";
                    if (message.policySnapshot != null && message.hasOwnProperty("policySnapshot"))
                        if (!$util.isString(message.policySnapshot))
                            return "policySnapshot: string expected";
                    if (message.ruleId != null && message.hasOwnProperty("ruleId"))
                        if (!$util.isString(message.ruleId))
                            return "ruleId: string expected";
                    if (message.constraints != null && message.hasOwnProperty("constraints")) {
                        var error = $root.cordum.agent.v1.PolicyConstraints.verify(message.constraints);
                        if (error)
                            return "constraints." + error;
                    }
                    if (message.approvalRequired != null && message.hasOwnProperty("approvalRequired"))
                        if (typeof message.approvalRequired !== "boolean")
                            return "approvalRequired: boolean expected";
                    if (message.approvalRef != null && message.hasOwnProperty("approvalRef"))
                        if (!$util.isString(message.approvalRef))
                            return "approvalRef: string expected";
                    return null;
                };

                /**
                 * Creates a PolicyCheckResponse message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.PolicyCheckResponse} PolicyCheckResponse
                 */
                PolicyCheckResponse.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.PolicyCheckResponse)
                        return object;
                    var message = new $root.cordum.agent.v1.PolicyCheckResponse();
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
                    case "DECISION_TYPE_ALLOW_WITH_CONSTRAINTS":
                    case 5:
                        message.decision = 5;
                        break;
                    }
                    if (object.reason != null)
                        message.reason = String(object.reason);
                    if (object.redactedContextPtr != null)
                        message.redactedContextPtr = String(object.redactedContextPtr);
                    if (object.policySnapshot != null)
                        message.policySnapshot = String(object.policySnapshot);
                    if (object.ruleId != null)
                        message.ruleId = String(object.ruleId);
                    if (object.constraints != null) {
                        if (typeof object.constraints !== "object")
                            throw TypeError(".cordum.agent.v1.PolicyCheckResponse.constraints: object expected");
                        message.constraints = $root.cordum.agent.v1.PolicyConstraints.fromObject(object.constraints);
                    }
                    if (object.approvalRequired != null)
                        message.approvalRequired = Boolean(object.approvalRequired);
                    if (object.approvalRef != null)
                        message.approvalRef = String(object.approvalRef);
                    return message;
                };

                /**
                 * Creates a plain object from a PolicyCheckResponse message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {cordum.agent.v1.PolicyCheckResponse} message PolicyCheckResponse
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
                        object.policySnapshot = "";
                        object.ruleId = "";
                        object.constraints = null;
                        object.approvalRequired = false;
                        object.approvalRef = "";
                    }
                    if (message.decision != null && message.hasOwnProperty("decision"))
                        object.decision = options.enums === String ? $root.cordum.agent.v1.DecisionType[message.decision] === undefined ? message.decision : $root.cordum.agent.v1.DecisionType[message.decision] : message.decision;
                    if (message.reason != null && message.hasOwnProperty("reason"))
                        object.reason = message.reason;
                    if (message.redactedContextPtr != null && message.hasOwnProperty("redactedContextPtr"))
                        object.redactedContextPtr = message.redactedContextPtr;
                    if (message.policySnapshot != null && message.hasOwnProperty("policySnapshot"))
                        object.policySnapshot = message.policySnapshot;
                    if (message.ruleId != null && message.hasOwnProperty("ruleId"))
                        object.ruleId = message.ruleId;
                    if (message.constraints != null && message.hasOwnProperty("constraints"))
                        object.constraints = $root.cordum.agent.v1.PolicyConstraints.toObject(message.constraints, options);
                    if (message.approvalRequired != null && message.hasOwnProperty("approvalRequired"))
                        object.approvalRequired = message.approvalRequired;
                    if (message.approvalRef != null && message.hasOwnProperty("approvalRef"))
                        object.approvalRef = message.approvalRef;
                    return object;
                };

                /**
                 * Converts this PolicyCheckResponse to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                PolicyCheckResponse.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for PolicyCheckResponse
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.PolicyCheckResponse
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                PolicyCheckResponse.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.PolicyCheckResponse";
                };

                return PolicyCheckResponse;
            })();

            v1.ListSnapshotsRequest = (function() {

                /**
                 * Properties of a ListSnapshotsRequest.
                 * @memberof cordum.agent.v1
                 * @interface IListSnapshotsRequest
                 */

                /**
                 * Constructs a new ListSnapshotsRequest.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a ListSnapshotsRequest.
                 * @implements IListSnapshotsRequest
                 * @constructor
                 * @param {cordum.agent.v1.IListSnapshotsRequest=} [properties] Properties to set
                 */
                function ListSnapshotsRequest(properties) {
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * Creates a new ListSnapshotsRequest instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @static
                 * @param {cordum.agent.v1.IListSnapshotsRequest=} [properties] Properties to set
                 * @returns {cordum.agent.v1.ListSnapshotsRequest} ListSnapshotsRequest instance
                 */
                ListSnapshotsRequest.create = function create(properties) {
                    return new ListSnapshotsRequest(properties);
                };

                /**
                 * Encodes the specified ListSnapshotsRequest message. Does not implicitly {@link cordum.agent.v1.ListSnapshotsRequest.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @static
                 * @param {cordum.agent.v1.IListSnapshotsRequest} message ListSnapshotsRequest message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ListSnapshotsRequest.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    return writer;
                };

                /**
                 * Encodes the specified ListSnapshotsRequest message, length delimited. Does not implicitly {@link cordum.agent.v1.ListSnapshotsRequest.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @static
                 * @param {cordum.agent.v1.IListSnapshotsRequest} message ListSnapshotsRequest message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ListSnapshotsRequest.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a ListSnapshotsRequest message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.ListSnapshotsRequest} ListSnapshotsRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ListSnapshotsRequest.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.ListSnapshotsRequest();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a ListSnapshotsRequest message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.ListSnapshotsRequest} ListSnapshotsRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ListSnapshotsRequest.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a ListSnapshotsRequest message.
                 * @function verify
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                ListSnapshotsRequest.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    return null;
                };

                /**
                 * Creates a ListSnapshotsRequest message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.ListSnapshotsRequest} ListSnapshotsRequest
                 */
                ListSnapshotsRequest.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.ListSnapshotsRequest)
                        return object;
                    return new $root.cordum.agent.v1.ListSnapshotsRequest();
                };

                /**
                 * Creates a plain object from a ListSnapshotsRequest message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @static
                 * @param {cordum.agent.v1.ListSnapshotsRequest} message ListSnapshotsRequest
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                ListSnapshotsRequest.toObject = function toObject() {
                    return {};
                };

                /**
                 * Converts this ListSnapshotsRequest to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                ListSnapshotsRequest.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for ListSnapshotsRequest
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.ListSnapshotsRequest
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                ListSnapshotsRequest.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.ListSnapshotsRequest";
                };

                return ListSnapshotsRequest;
            })();

            v1.ListSnapshotsResponse = (function() {

                /**
                 * Properties of a ListSnapshotsResponse.
                 * @memberof cordum.agent.v1
                 * @interface IListSnapshotsResponse
                 * @property {Array.<string>|null} [snapshots] ListSnapshotsResponse snapshots
                 */

                /**
                 * Constructs a new ListSnapshotsResponse.
                 * @memberof cordum.agent.v1
                 * @classdesc Represents a ListSnapshotsResponse.
                 * @implements IListSnapshotsResponse
                 * @constructor
                 * @param {cordum.agent.v1.IListSnapshotsResponse=} [properties] Properties to set
                 */
                function ListSnapshotsResponse(properties) {
                    this.snapshots = [];
                    if (properties)
                        for (var keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * ListSnapshotsResponse snapshots.
                 * @member {Array.<string>} snapshots
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @instance
                 */
                ListSnapshotsResponse.prototype.snapshots = $util.emptyArray;

                /**
                 * Creates a new ListSnapshotsResponse instance using the specified properties.
                 * @function create
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @static
                 * @param {cordum.agent.v1.IListSnapshotsResponse=} [properties] Properties to set
                 * @returns {cordum.agent.v1.ListSnapshotsResponse} ListSnapshotsResponse instance
                 */
                ListSnapshotsResponse.create = function create(properties) {
                    return new ListSnapshotsResponse(properties);
                };

                /**
                 * Encodes the specified ListSnapshotsResponse message. Does not implicitly {@link cordum.agent.v1.ListSnapshotsResponse.verify|verify} messages.
                 * @function encode
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @static
                 * @param {cordum.agent.v1.IListSnapshotsResponse} message ListSnapshotsResponse message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ListSnapshotsResponse.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.snapshots != null && message.snapshots.length)
                        for (var i = 0; i < message.snapshots.length; ++i)
                            writer.uint32(/* id 1, wireType 2 =*/10).string(message.snapshots[i]);
                    return writer;
                };

                /**
                 * Encodes the specified ListSnapshotsResponse message, length delimited. Does not implicitly {@link cordum.agent.v1.ListSnapshotsResponse.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @static
                 * @param {cordum.agent.v1.IListSnapshotsResponse} message ListSnapshotsResponse message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ListSnapshotsResponse.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a ListSnapshotsResponse message from the specified reader or buffer.
                 * @function decode
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {cordum.agent.v1.ListSnapshotsResponse} ListSnapshotsResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ListSnapshotsResponse.decode = function decode(reader, length, error) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    var end = length === undefined ? reader.len : reader.pos + length, message = new $root.cordum.agent.v1.ListSnapshotsResponse();
                    while (reader.pos < end) {
                        var tag = reader.uint32();
                        if (tag === error)
                            break;
                        switch (tag >>> 3) {
                        case 1: {
                                if (!(message.snapshots && message.snapshots.length))
                                    message.snapshots = [];
                                message.snapshots.push(reader.string());
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
                 * Decodes a ListSnapshotsResponse message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {cordum.agent.v1.ListSnapshotsResponse} ListSnapshotsResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ListSnapshotsResponse.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a ListSnapshotsResponse message.
                 * @function verify
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                ListSnapshotsResponse.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.snapshots != null && message.hasOwnProperty("snapshots")) {
                        if (!Array.isArray(message.snapshots))
                            return "snapshots: array expected";
                        for (var i = 0; i < message.snapshots.length; ++i)
                            if (!$util.isString(message.snapshots[i]))
                                return "snapshots: string[] expected";
                    }
                    return null;
                };

                /**
                 * Creates a ListSnapshotsResponse message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {cordum.agent.v1.ListSnapshotsResponse} ListSnapshotsResponse
                 */
                ListSnapshotsResponse.fromObject = function fromObject(object) {
                    if (object instanceof $root.cordum.agent.v1.ListSnapshotsResponse)
                        return object;
                    var message = new $root.cordum.agent.v1.ListSnapshotsResponse();
                    if (object.snapshots) {
                        if (!Array.isArray(object.snapshots))
                            throw TypeError(".cordum.agent.v1.ListSnapshotsResponse.snapshots: array expected");
                        message.snapshots = [];
                        for (var i = 0; i < object.snapshots.length; ++i)
                            message.snapshots[i] = String(object.snapshots[i]);
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a ListSnapshotsResponse message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @static
                 * @param {cordum.agent.v1.ListSnapshotsResponse} message ListSnapshotsResponse
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                ListSnapshotsResponse.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    var object = {};
                    if (options.arrays || options.defaults)
                        object.snapshots = [];
                    if (message.snapshots && message.snapshots.length) {
                        object.snapshots = [];
                        for (var j = 0; j < message.snapshots.length; ++j)
                            object.snapshots[j] = message.snapshots[j];
                    }
                    return object;
                };

                /**
                 * Converts this ListSnapshotsResponse to JSON.
                 * @function toJSON
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                ListSnapshotsResponse.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for ListSnapshotsResponse
                 * @function getTypeUrl
                 * @memberof cordum.agent.v1.ListSnapshotsResponse
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                ListSnapshotsResponse.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/cordum.agent.v1.ListSnapshotsResponse";
                };

                return ListSnapshotsResponse;
            })();

            v1.SafetyKernel = (function() {

                /**
                 * Constructs a new SafetyKernel service.
                 * @memberof cordum.agent.v1
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
                 * @memberof cordum.agent.v1.SafetyKernel
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
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#check}.
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @typedef CheckCallback
                 * @type {function}
                 * @param {Error|null} error Error, if any
                 * @param {cordum.agent.v1.PolicyCheckResponse} [response] PolicyCheckResponse
                 */

                /**
                 * Calls Check.
                 * @function check
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @param {cordum.agent.v1.SafetyKernel.CheckCallback} callback Node-style callback called with the error, if any, and PolicyCheckResponse
                 * @returns {undefined}
                 * @variation 1
                 */
                Object.defineProperty(SafetyKernel.prototype.check = function check(request, callback) {
                    return this.rpcCall(check, $root.cordum.agent.v1.PolicyCheckRequest, $root.cordum.agent.v1.PolicyCheckResponse, request, callback);
                }, "name", { value: "Check" });

                /**
                 * Calls Check.
                 * @function check
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @returns {Promise<cordum.agent.v1.PolicyCheckResponse>} Promise
                 * @variation 2
                 */

                /**
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#evaluate}.
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @typedef EvaluateCallback
                 * @type {function}
                 * @param {Error|null} error Error, if any
                 * @param {cordum.agent.v1.PolicyCheckResponse} [response] PolicyCheckResponse
                 */

                /**
                 * Calls Evaluate.
                 * @function evaluate
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @param {cordum.agent.v1.SafetyKernel.EvaluateCallback} callback Node-style callback called with the error, if any, and PolicyCheckResponse
                 * @returns {undefined}
                 * @variation 1
                 */
                Object.defineProperty(SafetyKernel.prototype.evaluate = function evaluate(request, callback) {
                    return this.rpcCall(evaluate, $root.cordum.agent.v1.PolicyCheckRequest, $root.cordum.agent.v1.PolicyCheckResponse, request, callback);
                }, "name", { value: "Evaluate" });

                /**
                 * Calls Evaluate.
                 * @function evaluate
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @returns {Promise<cordum.agent.v1.PolicyCheckResponse>} Promise
                 * @variation 2
                 */

                /**
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#explain}.
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @typedef ExplainCallback
                 * @type {function}
                 * @param {Error|null} error Error, if any
                 * @param {cordum.agent.v1.PolicyCheckResponse} [response] PolicyCheckResponse
                 */

                /**
                 * Calls Explain.
                 * @function explain
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @param {cordum.agent.v1.SafetyKernel.ExplainCallback} callback Node-style callback called with the error, if any, and PolicyCheckResponse
                 * @returns {undefined}
                 * @variation 1
                 */
                Object.defineProperty(SafetyKernel.prototype.explain = function explain(request, callback) {
                    return this.rpcCall(explain, $root.cordum.agent.v1.PolicyCheckRequest, $root.cordum.agent.v1.PolicyCheckResponse, request, callback);
                }, "name", { value: "Explain" });

                /**
                 * Calls Explain.
                 * @function explain
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @returns {Promise<cordum.agent.v1.PolicyCheckResponse>} Promise
                 * @variation 2
                 */

                /**
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#simulate}.
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @typedef SimulateCallback
                 * @type {function}
                 * @param {Error|null} error Error, if any
                 * @param {cordum.agent.v1.PolicyCheckResponse} [response] PolicyCheckResponse
                 */

                /**
                 * Calls Simulate.
                 * @function simulate
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @param {cordum.agent.v1.SafetyKernel.SimulateCallback} callback Node-style callback called with the error, if any, and PolicyCheckResponse
                 * @returns {undefined}
                 * @variation 1
                 */
                Object.defineProperty(SafetyKernel.prototype.simulate = function simulate(request, callback) {
                    return this.rpcCall(simulate, $root.cordum.agent.v1.PolicyCheckRequest, $root.cordum.agent.v1.PolicyCheckResponse, request, callback);
                }, "name", { value: "Simulate" });

                /**
                 * Calls Simulate.
                 * @function simulate
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IPolicyCheckRequest} request PolicyCheckRequest message or plain object
                 * @returns {Promise<cordum.agent.v1.PolicyCheckResponse>} Promise
                 * @variation 2
                 */

                /**
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#listSnapshots}.
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @typedef ListSnapshotsCallback
                 * @type {function}
                 * @param {Error|null} error Error, if any
                 * @param {cordum.agent.v1.ListSnapshotsResponse} [response] ListSnapshotsResponse
                 */

                /**
                 * Calls ListSnapshots.
                 * @function listSnapshots
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IListSnapshotsRequest} request ListSnapshotsRequest message or plain object
                 * @param {cordum.agent.v1.SafetyKernel.ListSnapshotsCallback} callback Node-style callback called with the error, if any, and ListSnapshotsResponse
                 * @returns {undefined}
                 * @variation 1
                 */
                Object.defineProperty(SafetyKernel.prototype.listSnapshots = function listSnapshots(request, callback) {
                    return this.rpcCall(listSnapshots, $root.cordum.agent.v1.ListSnapshotsRequest, $root.cordum.agent.v1.ListSnapshotsResponse, request, callback);
                }, "name", { value: "ListSnapshots" });

                /**
                 * Calls ListSnapshots.
                 * @function listSnapshots
                 * @memberof cordum.agent.v1.SafetyKernel
                 * @instance
                 * @param {cordum.agent.v1.IListSnapshotsRequest} request ListSnapshotsRequest message or plain object
                 * @returns {Promise<cordum.agent.v1.ListSnapshotsResponse>} Promise
                 * @variation 2
                 */

                return SafetyKernel;
            })();

            return v1;
        })();

        return agent;
    })();

    return cordum;
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
