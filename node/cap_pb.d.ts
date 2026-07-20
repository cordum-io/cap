import * as $protobuf from "protobufjs";
import Long = require("long");
/** Namespace cordum. */
export namespace cordum {

    /** Namespace agent. */
    namespace agent {

        /** Namespace v1. */
        namespace v1 {

            /** Properties of a BusPacket. */
            interface IBusPacket {

                /** BusPacket traceId */
                traceId?: (string|null);

                /** BusPacket senderId */
                senderId?: (string|null);

                /** BusPacket createdAt */
                createdAt?: (google.protobuf.ITimestamp|null);

                /** BusPacket protocolVersion */
                protocolVersion?: (number|null);

                /** BusPacket jobRequest */
                jobRequest?: (cordum.agent.v1.IJobRequest|null);

                /** BusPacket jobResult */
                jobResult?: (cordum.agent.v1.IJobResult|null);

                /** BusPacket heartbeat */
                heartbeat?: (cordum.agent.v1.IHeartbeat|null);

                /** BusPacket alert */
                alert?: (cordum.agent.v1.ISystemAlert|null);

                /** BusPacket jobProgress */
                jobProgress?: (cordum.agent.v1.IJobProgress|null);

                /** BusPacket jobCancel */
                jobCancel?: (cordum.agent.v1.IJobCancel|null);

                /** BusPacket handshake */
                handshake?: (cordum.agent.v1.IHandshake|null);

                /** BusPacket workerHandshakeChallengeRequest */
                workerHandshakeChallengeRequest?: (cordum.agent.v1.IWorkerHandshakeChallengeRequest|null);

                /** BusPacket workerHandshakeChallenge */
                workerHandshakeChallenge?: (cordum.agent.v1.IWorkerHandshakeChallenge|null);

                /** BusPacket workerHandshakeAuthenticate */
                workerHandshakeAuthenticate?: (cordum.agent.v1.IWorkerHandshakeAuthenticate|null);

                /** BusPacket workerHandshakeResult */
                workerHandshakeResult?: (cordum.agent.v1.IWorkerHandshakeResult|null);

                /** BusPacket signature */
                signature?: (Uint8Array|null);

                /** BusPacket authToken */
                authToken?: (string|null);
            }

            /** Represents a BusPacket. */
            class BusPacket implements IBusPacket {

                /**
                 * Constructs a new BusPacket.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IBusPacket);

                /** BusPacket traceId. */
                public traceId: string;

                /** BusPacket senderId. */
                public senderId: string;

                /** BusPacket createdAt. */
                public createdAt?: (google.protobuf.ITimestamp|null);

                /** BusPacket protocolVersion. */
                public protocolVersion: number;

                /** BusPacket jobRequest. */
                public jobRequest?: (cordum.agent.v1.IJobRequest|null);

                /** BusPacket jobResult. */
                public jobResult?: (cordum.agent.v1.IJobResult|null);

                /** BusPacket heartbeat. */
                public heartbeat?: (cordum.agent.v1.IHeartbeat|null);

                /** BusPacket alert. */
                public alert?: (cordum.agent.v1.ISystemAlert|null);

                /** BusPacket jobProgress. */
                public jobProgress?: (cordum.agent.v1.IJobProgress|null);

                /** BusPacket jobCancel. */
                public jobCancel?: (cordum.agent.v1.IJobCancel|null);

                /** BusPacket handshake. */
                public handshake?: (cordum.agent.v1.IHandshake|null);

                /** BusPacket workerHandshakeChallengeRequest. */
                public workerHandshakeChallengeRequest?: (cordum.agent.v1.IWorkerHandshakeChallengeRequest|null);

                /** BusPacket workerHandshakeChallenge. */
                public workerHandshakeChallenge?: (cordum.agent.v1.IWorkerHandshakeChallenge|null);

                /** BusPacket workerHandshakeAuthenticate. */
                public workerHandshakeAuthenticate?: (cordum.agent.v1.IWorkerHandshakeAuthenticate|null);

                /** BusPacket workerHandshakeResult. */
                public workerHandshakeResult?: (cordum.agent.v1.IWorkerHandshakeResult|null);

                /** BusPacket signature. */
                public signature: Uint8Array;

                /** BusPacket authToken. */
                public authToken: string;

                /** BusPacket payload. */
                public payload?: ("jobRequest"|"jobResult"|"heartbeat"|"alert"|"jobProgress"|"jobCancel"|"handshake"|"workerHandshakeChallengeRequest"|"workerHandshakeChallenge"|"workerHandshakeAuthenticate"|"workerHandshakeResult");

                /**
                 * Creates a new BusPacket instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns BusPacket instance
                 */
                public static create(properties?: cordum.agent.v1.IBusPacket): cordum.agent.v1.BusPacket;

                /**
                 * Encodes the specified BusPacket message. Does not implicitly {@link cordum.agent.v1.BusPacket.verify|verify} messages.
                 * @param message BusPacket message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IBusPacket, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified BusPacket message, length delimited. Does not implicitly {@link cordum.agent.v1.BusPacket.verify|verify} messages.
                 * @param message BusPacket message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IBusPacket, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a BusPacket message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns BusPacket
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.BusPacket;

                /**
                 * Decodes a BusPacket message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns BusPacket
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.BusPacket;

                /**
                 * Verifies a BusPacket message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a BusPacket message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns BusPacket
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.BusPacket;

                /**
                 * Creates a plain object from a BusPacket message. Also converts values to other types if specified.
                 * @param message BusPacket
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.BusPacket, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this BusPacket to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for BusPacket
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** JobPriority enum. */
            enum JobPriority {
                JOB_PRIORITY_UNSPECIFIED = 0,
                JOB_PRIORITY_INTERACTIVE = 1,
                JOB_PRIORITY_BATCH = 2,
                JOB_PRIORITY_CRITICAL = 3
            }

            /** JobStatus enum. */
            enum JobStatus {
                JOB_STATUS_UNSPECIFIED = 0,
                JOB_STATUS_PENDING = 1,
                JOB_STATUS_SCHEDULED = 2,
                JOB_STATUS_DISPATCHED = 3,
                JOB_STATUS_RUNNING = 4,
                JOB_STATUS_SUCCEEDED = 5,
                JOB_STATUS_FAILED = 6,
                JOB_STATUS_CANCELLED = 7,
                JOB_STATUS_DENIED = 8,
                JOB_STATUS_TIMEOUT = 9,
                JOB_STATUS_FAILED_RETRYABLE = 10,
                JOB_STATUS_FAILED_FATAL = 11
            }

            /** Properties of a ContextHints. */
            interface IContextHints {

                /** ContextHints maxInputTokens */
                maxInputTokens?: (number|null);

                /** ContextHints allowSummarization */
                allowSummarization?: (boolean|null);

                /** ContextHints allowRetrieval */
                allowRetrieval?: (boolean|null);

                /** ContextHints tags */
                tags?: (string[]|null);
            }

            /** Represents a ContextHints. */
            class ContextHints implements IContextHints {

                /**
                 * Constructs a new ContextHints.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IContextHints);

                /** ContextHints maxInputTokens. */
                public maxInputTokens: number;

                /** ContextHints allowSummarization. */
                public allowSummarization: boolean;

                /** ContextHints allowRetrieval. */
                public allowRetrieval: boolean;

                /** ContextHints tags. */
                public tags: string[];

                /**
                 * Creates a new ContextHints instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns ContextHints instance
                 */
                public static create(properties?: cordum.agent.v1.IContextHints): cordum.agent.v1.ContextHints;

                /**
                 * Encodes the specified ContextHints message. Does not implicitly {@link cordum.agent.v1.ContextHints.verify|verify} messages.
                 * @param message ContextHints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IContextHints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified ContextHints message, length delimited. Does not implicitly {@link cordum.agent.v1.ContextHints.verify|verify} messages.
                 * @param message ContextHints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IContextHints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a ContextHints message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns ContextHints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.ContextHints;

                /**
                 * Decodes a ContextHints message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns ContextHints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.ContextHints;

                /**
                 * Verifies a ContextHints message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a ContextHints message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns ContextHints
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.ContextHints;

                /**
                 * Creates a plain object from a ContextHints message. Also converts values to other types if specified.
                 * @param message ContextHints
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.ContextHints, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this ContextHints to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for ContextHints
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a Budget. */
            interface IBudget {

                /** Budget maxInputTokens */
                maxInputTokens?: (number|Long|null);

                /** Budget maxOutputTokens */
                maxOutputTokens?: (number|Long|null);

                /** Budget maxTotalTokens */
                maxTotalTokens?: (number|Long|null);

                /** Budget deadlineMs */
                deadlineMs?: (number|Long|null);
            }

            /** Represents a Budget. */
            class Budget implements IBudget {

                /**
                 * Constructs a new Budget.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IBudget);

                /** Budget maxInputTokens. */
                public maxInputTokens: (number|Long);

                /** Budget maxOutputTokens. */
                public maxOutputTokens: (number|Long);

                /** Budget maxTotalTokens. */
                public maxTotalTokens: (number|Long);

                /** Budget deadlineMs. */
                public deadlineMs: (number|Long);

                /**
                 * Creates a new Budget instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns Budget instance
                 */
                public static create(properties?: cordum.agent.v1.IBudget): cordum.agent.v1.Budget;

                /**
                 * Encodes the specified Budget message. Does not implicitly {@link cordum.agent.v1.Budget.verify|verify} messages.
                 * @param message Budget message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IBudget, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified Budget message, length delimited. Does not implicitly {@link cordum.agent.v1.Budget.verify|verify} messages.
                 * @param message Budget message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IBudget, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a Budget message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns Budget
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.Budget;

                /**
                 * Decodes a Budget message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns Budget
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.Budget;

                /**
                 * Verifies a Budget message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a Budget message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns Budget
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.Budget;

                /**
                 * Creates a plain object from a Budget message. Also converts values to other types if specified.
                 * @param message Budget
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.Budget, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this Budget to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for Budget
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** ActorType enum. */
            enum ActorType {
                ACTOR_TYPE_UNSPECIFIED = 0,
                ACTOR_TYPE_HUMAN = 1,
                ACTOR_TYPE_SERVICE = 2
            }

            /** ErrorCode enum. */
            enum ErrorCode {
                ERROR_CODE_UNSPECIFIED = 0,
                ERROR_CODE_PROTOCOL_VERSION_MISMATCH = 100,
                ERROR_CODE_PROTOCOL_MALFORMED_PACKET = 101,
                ERROR_CODE_PROTOCOL_UNKNOWN_PAYLOAD = 102,
                ERROR_CODE_PROTOCOL_SIGNATURE_INVALID = 103,
                ERROR_CODE_PROTOCOL_SIGNATURE_MISSING = 104,
                ERROR_CODE_JOB_TIMEOUT = 200,
                ERROR_CODE_JOB_RESOURCE_EXHAUSTED = 201,
                ERROR_CODE_JOB_PERMISSION_DENIED = 202,
                ERROR_CODE_JOB_INVALID_INPUT = 203,
                ERROR_CODE_JOB_NOT_FOUND = 204,
                ERROR_CODE_JOB_DUPLICATE = 205,
                ERROR_CODE_JOB_WORKER_UNAVAILABLE = 206,
                ERROR_CODE_SAFETY_DENIED = 300,
                ERROR_CODE_SAFETY_POLICY_VIOLATION = 301,
                ERROR_CODE_SAFETY_RISK_TAG_BLOCKED = 302,
                ERROR_CODE_TRANSPORT_PUBLISH_FAILED = 400,
                ERROR_CODE_TRANSPORT_SUBSCRIBE_FAILED = 401,
                ERROR_CODE_TRANSPORT_CONNECTION_LOST = 402
            }

            /** Properties of a JobMetadata. */
            interface IJobMetadata {

                /** JobMetadata tenantId */
                tenantId?: (string|null);

                /** JobMetadata actorId */
                actorId?: (string|null);

                /** JobMetadata actorType */
                actorType?: (cordum.agent.v1.ActorType|null);

                /** JobMetadata idempotencyKey */
                idempotencyKey?: (string|null);

                /** JobMetadata capability */
                capability?: (string|null);

                /** JobMetadata riskTags */
                riskTags?: (string[]|null);

                /** JobMetadata requires */
                requires?: (string[]|null);

                /** JobMetadata packId */
                packId?: (string|null);

                /** JobMetadata labels */
                labels?: ({ [k: string]: string }|null);
            }

            /** Represents a JobMetadata. */
            class JobMetadata implements IJobMetadata {

                /**
                 * Constructs a new JobMetadata.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IJobMetadata);

                /** JobMetadata tenantId. */
                public tenantId: string;

                /** JobMetadata actorId. */
                public actorId: string;

                /** JobMetadata actorType. */
                public actorType: cordum.agent.v1.ActorType;

                /** JobMetadata idempotencyKey. */
                public idempotencyKey: string;

                /** JobMetadata capability. */
                public capability: string;

                /** JobMetadata riskTags. */
                public riskTags: string[];

                /** JobMetadata requires. */
                public requires: string[];

                /** JobMetadata packId. */
                public packId: string;

                /** JobMetadata labels. */
                public labels: { [k: string]: string };

                /**
                 * Creates a new JobMetadata instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns JobMetadata instance
                 */
                public static create(properties?: cordum.agent.v1.IJobMetadata): cordum.agent.v1.JobMetadata;

                /**
                 * Encodes the specified JobMetadata message. Does not implicitly {@link cordum.agent.v1.JobMetadata.verify|verify} messages.
                 * @param message JobMetadata message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IJobMetadata, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified JobMetadata message, length delimited. Does not implicitly {@link cordum.agent.v1.JobMetadata.verify|verify} messages.
                 * @param message JobMetadata message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IJobMetadata, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a JobMetadata message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns JobMetadata
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.JobMetadata;

                /**
                 * Decodes a JobMetadata message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns JobMetadata
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.JobMetadata;

                /**
                 * Verifies a JobMetadata message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a JobMetadata message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns JobMetadata
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.JobMetadata;

                /**
                 * Creates a plain object from a JobMetadata message. Also converts values to other types if specified.
                 * @param message JobMetadata
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.JobMetadata, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this JobMetadata to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for JobMetadata
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a Compensation. */
            interface ICompensation {

                /** Compensation topic */
                topic?: (string|null);

                /** Compensation contextPtr */
                contextPtr?: (string|null);

                /** Compensation priority */
                priority?: (cordum.agent.v1.JobPriority|null);

                /** Compensation adapterId */
                adapterId?: (string|null);

                /** Compensation env */
                env?: ({ [k: string]: string }|null);

                /** Compensation memoryId */
                memoryId?: (string|null);

                /** Compensation contextHints */
                contextHints?: (cordum.agent.v1.IContextHints|null);

                /** Compensation budget */
                budget?: (cordum.agent.v1.IBudget|null);

                /** Compensation tenantId */
                tenantId?: (string|null);

                /** Compensation principalId */
                principalId?: (string|null);

                /** Compensation labels */
                labels?: ({ [k: string]: string }|null);

                /** Compensation meta */
                meta?: (cordum.agent.v1.IJobMetadata|null);
            }

            /** Represents a Compensation. */
            class Compensation implements ICompensation {

                /**
                 * Constructs a new Compensation.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.ICompensation);

                /** Compensation topic. */
                public topic: string;

                /** Compensation contextPtr. */
                public contextPtr: string;

                /** Compensation priority. */
                public priority: cordum.agent.v1.JobPriority;

                /** Compensation adapterId. */
                public adapterId: string;

                /** Compensation env. */
                public env: { [k: string]: string };

                /** Compensation memoryId. */
                public memoryId: string;

                /** Compensation contextHints. */
                public contextHints?: (cordum.agent.v1.IContextHints|null);

                /** Compensation budget. */
                public budget?: (cordum.agent.v1.IBudget|null);

                /** Compensation tenantId. */
                public tenantId: string;

                /** Compensation principalId. */
                public principalId: string;

                /** Compensation labels. */
                public labels: { [k: string]: string };

                /** Compensation meta. */
                public meta?: (cordum.agent.v1.IJobMetadata|null);

                /**
                 * Creates a new Compensation instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns Compensation instance
                 */
                public static create(properties?: cordum.agent.v1.ICompensation): cordum.agent.v1.Compensation;

                /**
                 * Encodes the specified Compensation message. Does not implicitly {@link cordum.agent.v1.Compensation.verify|verify} messages.
                 * @param message Compensation message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.ICompensation, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified Compensation message, length delimited. Does not implicitly {@link cordum.agent.v1.Compensation.verify|verify} messages.
                 * @param message Compensation message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.ICompensation, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a Compensation message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns Compensation
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.Compensation;

                /**
                 * Decodes a Compensation message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns Compensation
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.Compensation;

                /**
                 * Verifies a Compensation message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a Compensation message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns Compensation
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.Compensation;

                /**
                 * Creates a plain object from a Compensation message. Also converts values to other types if specified.
                 * @param message Compensation
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.Compensation, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this Compensation to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for Compensation
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a JobRequest. */
            interface IJobRequest {

                /** JobRequest jobId */
                jobId?: (string|null);

                /** JobRequest topic */
                topic?: (string|null);

                /** JobRequest priority */
                priority?: (cordum.agent.v1.JobPriority|null);

                /** JobRequest contextPtr */
                contextPtr?: (string|null);

                /** JobRequest adapterId */
                adapterId?: (string|null);

                /** JobRequest env */
                env?: ({ [k: string]: string }|null);

                /** JobRequest parentJobId */
                parentJobId?: (string|null);

                /** JobRequest workflowId */
                workflowId?: (string|null);

                /** JobRequest stepIndex */
                stepIndex?: (number|null);

                /** JobRequest memoryId */
                memoryId?: (string|null);

                /** JobRequest contextHints */
                contextHints?: (cordum.agent.v1.IContextHints|null);

                /** JobRequest budget */
                budget?: (cordum.agent.v1.IBudget|null);

                /** JobRequest tenantId */
                tenantId?: (string|null);

                /** JobRequest principalId */
                principalId?: (string|null);

                /** JobRequest labels */
                labels?: ({ [k: string]: string }|null);

                /** JobRequest meta */
                meta?: (cordum.agent.v1.IJobMetadata|null);

                /** JobRequest compensation */
                compensation?: (cordum.agent.v1.ICompensation|null);
            }

            /** Represents a JobRequest. */
            class JobRequest implements IJobRequest {

                /**
                 * Constructs a new JobRequest.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IJobRequest);

                /** JobRequest jobId. */
                public jobId: string;

                /** JobRequest topic. */
                public topic: string;

                /** JobRequest priority. */
                public priority: cordum.agent.v1.JobPriority;

                /** JobRequest contextPtr. */
                public contextPtr: string;

                /** JobRequest adapterId. */
                public adapterId: string;

                /** JobRequest env. */
                public env: { [k: string]: string };

                /** JobRequest parentJobId. */
                public parentJobId: string;

                /** JobRequest workflowId. */
                public workflowId: string;

                /** JobRequest stepIndex. */
                public stepIndex: number;

                /** JobRequest memoryId. */
                public memoryId: string;

                /** JobRequest contextHints. */
                public contextHints?: (cordum.agent.v1.IContextHints|null);

                /** JobRequest budget. */
                public budget?: (cordum.agent.v1.IBudget|null);

                /** JobRequest tenantId. */
                public tenantId: string;

                /** JobRequest principalId. */
                public principalId: string;

                /** JobRequest labels. */
                public labels: { [k: string]: string };

                /** JobRequest meta. */
                public meta?: (cordum.agent.v1.IJobMetadata|null);

                /** JobRequest compensation. */
                public compensation?: (cordum.agent.v1.ICompensation|null);

                /**
                 * Creates a new JobRequest instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns JobRequest instance
                 */
                public static create(properties?: cordum.agent.v1.IJobRequest): cordum.agent.v1.JobRequest;

                /**
                 * Encodes the specified JobRequest message. Does not implicitly {@link cordum.agent.v1.JobRequest.verify|verify} messages.
                 * @param message JobRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IJobRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified JobRequest message, length delimited. Does not implicitly {@link cordum.agent.v1.JobRequest.verify|verify} messages.
                 * @param message JobRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IJobRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a JobRequest message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns JobRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.JobRequest;

                /**
                 * Decodes a JobRequest message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns JobRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.JobRequest;

                /**
                 * Verifies a JobRequest message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a JobRequest message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns JobRequest
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.JobRequest;

                /**
                 * Creates a plain object from a JobRequest message. Also converts values to other types if specified.
                 * @param message JobRequest
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.JobRequest, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this JobRequest to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for JobRequest
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a JobResult. */
            interface IJobResult {

                /** JobResult jobId */
                jobId?: (string|null);

                /** JobResult status */
                status?: (cordum.agent.v1.JobStatus|null);

                /** JobResult resultPtr */
                resultPtr?: (string|null);

                /** JobResult workerId */
                workerId?: (string|null);

                /** JobResult executionMs */
                executionMs?: (number|Long|null);

                /** JobResult errorCode */
                errorCode?: (string|null);

                /** JobResult errorMessage */
                errorMessage?: (string|null);

                /** JobResult artifactPtrs */
                artifactPtrs?: (string[]|null);

                /** JobResult errorCodeEnum */
                errorCodeEnum?: (cordum.agent.v1.ErrorCode|null);
            }

            /** Represents a JobResult. */
            class JobResult implements IJobResult {

                /**
                 * Constructs a new JobResult.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IJobResult);

                /** JobResult jobId. */
                public jobId: string;

                /** JobResult status. */
                public status: cordum.agent.v1.JobStatus;

                /** JobResult resultPtr. */
                public resultPtr: string;

                /** JobResult workerId. */
                public workerId: string;

                /** JobResult executionMs. */
                public executionMs: (number|Long);

                /** JobResult errorCode. */
                public errorCode: string;

                /** JobResult errorMessage. */
                public errorMessage: string;

                /** JobResult artifactPtrs. */
                public artifactPtrs: string[];

                /** JobResult errorCodeEnum. */
                public errorCodeEnum: cordum.agent.v1.ErrorCode;

                /**
                 * Creates a new JobResult instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns JobResult instance
                 */
                public static create(properties?: cordum.agent.v1.IJobResult): cordum.agent.v1.JobResult;

                /**
                 * Encodes the specified JobResult message. Does not implicitly {@link cordum.agent.v1.JobResult.verify|verify} messages.
                 * @param message JobResult message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IJobResult, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified JobResult message, length delimited. Does not implicitly {@link cordum.agent.v1.JobResult.verify|verify} messages.
                 * @param message JobResult message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IJobResult, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a JobResult message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns JobResult
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.JobResult;

                /**
                 * Decodes a JobResult message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns JobResult
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.JobResult;

                /**
                 * Verifies a JobResult message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a JobResult message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns JobResult
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.JobResult;

                /**
                 * Creates a plain object from a JobResult message. Also converts values to other types if specified.
                 * @param message JobResult
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.JobResult, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this JobResult to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for JobResult
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a JobProgress. */
            interface IJobProgress {

                /** JobProgress jobId */
                jobId?: (string|null);

                /** JobProgress stepId */
                stepId?: (string|null);

                /** JobProgress percent */
                percent?: (number|null);

                /** JobProgress message */
                message?: (string|null);

                /** JobProgress resultPtr */
                resultPtr?: (string|null);

                /** JobProgress artifactPtrs */
                artifactPtrs?: (string[]|null);

                /** JobProgress status */
                status?: (cordum.agent.v1.JobStatus|null);
            }

            /** Represents a JobProgress. */
            class JobProgress implements IJobProgress {

                /**
                 * Constructs a new JobProgress.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IJobProgress);

                /** JobProgress jobId. */
                public jobId: string;

                /** JobProgress stepId. */
                public stepId: string;

                /** JobProgress percent. */
                public percent: number;

                /** JobProgress message. */
                public message: string;

                /** JobProgress resultPtr. */
                public resultPtr: string;

                /** JobProgress artifactPtrs. */
                public artifactPtrs: string[];

                /** JobProgress status. */
                public status: cordum.agent.v1.JobStatus;

                /**
                 * Creates a new JobProgress instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns JobProgress instance
                 */
                public static create(properties?: cordum.agent.v1.IJobProgress): cordum.agent.v1.JobProgress;

                /**
                 * Encodes the specified JobProgress message. Does not implicitly {@link cordum.agent.v1.JobProgress.verify|verify} messages.
                 * @param message JobProgress message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IJobProgress, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified JobProgress message, length delimited. Does not implicitly {@link cordum.agent.v1.JobProgress.verify|verify} messages.
                 * @param message JobProgress message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IJobProgress, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a JobProgress message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns JobProgress
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.JobProgress;

                /**
                 * Decodes a JobProgress message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns JobProgress
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.JobProgress;

                /**
                 * Verifies a JobProgress message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a JobProgress message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns JobProgress
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.JobProgress;

                /**
                 * Creates a plain object from a JobProgress message. Also converts values to other types if specified.
                 * @param message JobProgress
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.JobProgress, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this JobProgress to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for JobProgress
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a JobCancel. */
            interface IJobCancel {

                /** JobCancel jobId */
                jobId?: (string|null);

                /** JobCancel reason */
                reason?: (string|null);

                /** JobCancel requestedBy */
                requestedBy?: (string|null);
            }

            /** Represents a JobCancel. */
            class JobCancel implements IJobCancel {

                /**
                 * Constructs a new JobCancel.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IJobCancel);

                /** JobCancel jobId. */
                public jobId: string;

                /** JobCancel reason. */
                public reason: string;

                /** JobCancel requestedBy. */
                public requestedBy: string;

                /**
                 * Creates a new JobCancel instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns JobCancel instance
                 */
                public static create(properties?: cordum.agent.v1.IJobCancel): cordum.agent.v1.JobCancel;

                /**
                 * Encodes the specified JobCancel message. Does not implicitly {@link cordum.agent.v1.JobCancel.verify|verify} messages.
                 * @param message JobCancel message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IJobCancel, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified JobCancel message, length delimited. Does not implicitly {@link cordum.agent.v1.JobCancel.verify|verify} messages.
                 * @param message JobCancel message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IJobCancel, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a JobCancel message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns JobCancel
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.JobCancel;

                /**
                 * Decodes a JobCancel message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns JobCancel
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.JobCancel;

                /**
                 * Verifies a JobCancel message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a JobCancel message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns JobCancel
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.JobCancel;

                /**
                 * Creates a plain object from a JobCancel message. Also converts values to other types if specified.
                 * @param message JobCancel
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.JobCancel, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this JobCancel to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for JobCancel
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a Heartbeat. */
            interface IHeartbeat {

                /** Heartbeat workerId */
                workerId?: (string|null);

                /** Heartbeat region */
                region?: (string|null);

                /** Heartbeat type */
                type?: (string|null);

                /** Heartbeat cpuLoad */
                cpuLoad?: (number|null);

                /** Heartbeat gpuUtilization */
                gpuUtilization?: (number|null);

                /** Heartbeat activeJobs */
                activeJobs?: (number|null);

                /** Heartbeat capabilities */
                capabilities?: (string[]|null);

                /** Heartbeat pool */
                pool?: (string|null);

                /** Heartbeat maxParallelJobs */
                maxParallelJobs?: (number|null);

                /** Heartbeat labels */
                labels?: ({ [k: string]: string }|null);

                /** Heartbeat memoryLoad */
                memoryLoad?: (number|null);

                /** Heartbeat progressPct */
                progressPct?: (number|null);

                /** Heartbeat lastMemo */
                lastMemo?: (string|null);

                /** Heartbeat authToken */
                authToken?: (string|null);

                /** Heartbeat agentName */
                agentName?: (string|null);
            }

            /** Represents a Heartbeat. */
            class Heartbeat implements IHeartbeat {

                /**
                 * Constructs a new Heartbeat.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IHeartbeat);

                /** Heartbeat workerId. */
                public workerId: string;

                /** Heartbeat region. */
                public region: string;

                /** Heartbeat type. */
                public type: string;

                /** Heartbeat cpuLoad. */
                public cpuLoad: number;

                /** Heartbeat gpuUtilization. */
                public gpuUtilization: number;

                /** Heartbeat activeJobs. */
                public activeJobs: number;

                /** Heartbeat capabilities. */
                public capabilities: string[];

                /** Heartbeat pool. */
                public pool: string;

                /** Heartbeat maxParallelJobs. */
                public maxParallelJobs: number;

                /** Heartbeat labels. */
                public labels: { [k: string]: string };

                /** Heartbeat memoryLoad. */
                public memoryLoad: number;

                /** Heartbeat progressPct. */
                public progressPct: number;

                /** Heartbeat lastMemo. */
                public lastMemo: string;

                /** Heartbeat authToken. */
                public authToken: string;

                /** Heartbeat agentName. */
                public agentName: string;

                /**
                 * Creates a new Heartbeat instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns Heartbeat instance
                 */
                public static create(properties?: cordum.agent.v1.IHeartbeat): cordum.agent.v1.Heartbeat;

                /**
                 * Encodes the specified Heartbeat message. Does not implicitly {@link cordum.agent.v1.Heartbeat.verify|verify} messages.
                 * @param message Heartbeat message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IHeartbeat, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified Heartbeat message, length delimited. Does not implicitly {@link cordum.agent.v1.Heartbeat.verify|verify} messages.
                 * @param message Heartbeat message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IHeartbeat, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a Heartbeat message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns Heartbeat
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.Heartbeat;

                /**
                 * Decodes a Heartbeat message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns Heartbeat
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.Heartbeat;

                /**
                 * Verifies a Heartbeat message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a Heartbeat message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns Heartbeat
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.Heartbeat;

                /**
                 * Creates a plain object from a Heartbeat message. Also converts values to other types if specified.
                 * @param message Heartbeat
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.Heartbeat, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this Heartbeat to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for Heartbeat
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** AlertSeverity enum. */
            enum AlertSeverity {
                ALERT_SEVERITY_UNSPECIFIED = 0,
                ALERT_SEVERITY_INFO = 1,
                ALERT_SEVERITY_WARNING = 2,
                ALERT_SEVERITY_ERROR = 3,
                ALERT_SEVERITY_CRITICAL = 4
            }

            /** Properties of a SystemAlert. */
            interface ISystemAlert {

                /** SystemAlert level */
                level?: (string|null);

                /** SystemAlert message */
                message?: (string|null);

                /** SystemAlert component */
                component?: (string|null);

                /** SystemAlert code */
                code?: (string|null);

                /** SystemAlert severity */
                severity?: (cordum.agent.v1.AlertSeverity|null);

                /** SystemAlert errorCodeEnum */
                errorCodeEnum?: (cordum.agent.v1.ErrorCode|null);

                /** SystemAlert sourceComponent */
                sourceComponent?: (string|null);

                /** SystemAlert details */
                details?: ({ [k: string]: string }|null);

                /** SystemAlert traceId */
                traceId?: (string|null);
            }

            /** Represents a SystemAlert. */
            class SystemAlert implements ISystemAlert {

                /**
                 * Constructs a new SystemAlert.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.ISystemAlert);

                /** SystemAlert level. */
                public level: string;

                /** SystemAlert message. */
                public message: string;

                /** SystemAlert component. */
                public component: string;

                /** SystemAlert code. */
                public code: string;

                /** SystemAlert severity. */
                public severity: cordum.agent.v1.AlertSeverity;

                /** SystemAlert errorCodeEnum. */
                public errorCodeEnum: cordum.agent.v1.ErrorCode;

                /** SystemAlert sourceComponent. */
                public sourceComponent: string;

                /** SystemAlert details. */
                public details: { [k: string]: string };

                /** SystemAlert traceId. */
                public traceId: string;

                /**
                 * Creates a new SystemAlert instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns SystemAlert instance
                 */
                public static create(properties?: cordum.agent.v1.ISystemAlert): cordum.agent.v1.SystemAlert;

                /**
                 * Encodes the specified SystemAlert message. Does not implicitly {@link cordum.agent.v1.SystemAlert.verify|verify} messages.
                 * @param message SystemAlert message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.ISystemAlert, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified SystemAlert message, length delimited. Does not implicitly {@link cordum.agent.v1.SystemAlert.verify|verify} messages.
                 * @param message SystemAlert message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.ISystemAlert, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a SystemAlert message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns SystemAlert
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.SystemAlert;

                /**
                 * Decodes a SystemAlert message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns SystemAlert
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.SystemAlert;

                /**
                 * Verifies a SystemAlert message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a SystemAlert message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns SystemAlert
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.SystemAlert;

                /**
                 * Creates a plain object from a SystemAlert message. Also converts values to other types if specified.
                 * @param message SystemAlert
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.SystemAlert, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this SystemAlert to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for SystemAlert
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** ComponentRole enum. */
            enum ComponentRole {
                COMPONENT_ROLE_UNSPECIFIED = 0,
                COMPONENT_ROLE_GATEWAY = 1,
                COMPONENT_ROLE_SCHEDULER = 2,
                COMPONENT_ROLE_WORKER = 3,
                COMPONENT_ROLE_ORCHESTRATOR = 4,
                COMPONENT_ROLE_CONTROLLER = 5
            }

            /** Properties of a Handshake. */
            interface IHandshake {

                /** Handshake componentId */
                componentId?: (string|null);

                /** Handshake role */
                role?: (cordum.agent.v1.ComponentRole|null);

                /** Handshake supportedVersions */
                supportedVersions?: (number[]|null);

                /** Handshake capabilities */
                capabilities?: ({ [k: string]: boolean }|null);

                /** Handshake sdkVersion */
                sdkVersion?: (string|null);

                /** Handshake readyTopics */
                readyTopics?: (string[]|null);

                /** Handshake agentName */
                agentName?: (string|null);
            }

            /** Represents a Handshake. */
            class Handshake implements IHandshake {

                /**
                 * Constructs a new Handshake.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IHandshake);

                /** Handshake componentId. */
                public componentId: string;

                /** Handshake role. */
                public role: cordum.agent.v1.ComponentRole;

                /** Handshake supportedVersions. */
                public supportedVersions: number[];

                /** Handshake capabilities. */
                public capabilities: { [k: string]: boolean };

                /** Handshake sdkVersion. */
                public sdkVersion: string;

                /** Handshake readyTopics. */
                public readyTopics: string[];

                /** Handshake agentName. */
                public agentName: string;

                /**
                 * Creates a new Handshake instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns Handshake instance
                 */
                public static create(properties?: cordum.agent.v1.IHandshake): cordum.agent.v1.Handshake;

                /**
                 * Encodes the specified Handshake message. Does not implicitly {@link cordum.agent.v1.Handshake.verify|verify} messages.
                 * @param message Handshake message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IHandshake, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified Handshake message, length delimited. Does not implicitly {@link cordum.agent.v1.Handshake.verify|verify} messages.
                 * @param message Handshake message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IHandshake, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a Handshake message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns Handshake
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.Handshake;

                /**
                 * Decodes a Handshake message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns Handshake
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.Handshake;

                /**
                 * Verifies a Handshake message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a Handshake message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns Handshake
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.Handshake;

                /**
                 * Creates a plain object from a Handshake message. Also converts values to other types if specified.
                 * @param message Handshake
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.Handshake, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this Handshake to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for Handshake
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** WorkerHandshakePurpose enum. */
            enum WorkerHandshakePurpose {
                WORKER_HANDSHAKE_PURPOSE_UNSPECIFIED = 0,
                WORKER_HANDSHAKE_PURPOSE_ISSUE = 1,
                WORKER_HANDSHAKE_PURPOSE_RENEW = 2
            }

            /** WorkerHandshakeProofAlgorithm enum. */
            enum WorkerHandshakeProofAlgorithm {
                WORKER_HANDSHAKE_PROOF_ALGORITHM_UNSPECIFIED = 0,
                WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256 = 1
            }

            /** WorkerHandshakeRejectionReason enum. */
            enum WorkerHandshakeRejectionReason {
                WORKER_HANDSHAKE_REJECTION_REASON_UNSPECIFIED = 0,
                WORKER_HANDSHAKE_REJECTION_REASON_INVALID_REQUEST = 1,
                WORKER_HANDSHAKE_REJECTION_REASON_AUTHENTICATION_FAILED = 2,
                WORKER_HANDSHAKE_REJECTION_REASON_REPLAY_DETECTED = 3,
                WORKER_HANDSHAKE_REJECTION_REASON_CLOCK_SKEW = 4,
                WORKER_HANDSHAKE_REJECTION_REASON_CHALLENGE_EXPIRED = 5,
                WORKER_HANDSHAKE_REJECTION_REASON_SESSION_REQUIRED = 6,
                WORKER_HANDSHAKE_REJECTION_REASON_SESSION_INVALID = 7,
                WORKER_HANDSHAKE_REJECTION_REASON_UNSUPPORTED_VERSION = 8,
                WORKER_HANDSHAKE_REJECTION_REASON_INTERNAL_ERROR = 9
            }

            /** Properties of a WorkerHandshakeChallengeRequest. */
            interface IWorkerHandshakeChallengeRequest {

                /** WorkerHandshakeChallengeRequest requestId */
                requestId?: (string|null);

                /** WorkerHandshakeChallengeRequest traceId */
                traceId?: (string|null);

                /** WorkerHandshakeChallengeRequest workerId */
                workerId?: (string|null);

                /** WorkerHandshakeChallengeRequest proofKeyId */
                proofKeyId?: (string|null);

                /** WorkerHandshakeChallengeRequest proofAlgorithm */
                proofAlgorithm?: (cordum.agent.v1.WorkerHandshakeProofAlgorithm|null);

                /** WorkerHandshakeChallengeRequest audience */
                audience?: (string|null);

                /** WorkerHandshakeChallengeRequest purpose */
                purpose?: (cordum.agent.v1.WorkerHandshakePurpose|null);

                /** WorkerHandshakeChallengeRequest clientNonce */
                clientNonce?: (Uint8Array|null);

                /** WorkerHandshakeChallengeRequest protocolVersion */
                protocolVersion?: (number|null);

                /** WorkerHandshakeChallengeRequest sdkVersion */
                sdkVersion?: (string|null);
            }

            /** Represents a WorkerHandshakeChallengeRequest. */
            class WorkerHandshakeChallengeRequest implements IWorkerHandshakeChallengeRequest {

                /**
                 * Constructs a new WorkerHandshakeChallengeRequest.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IWorkerHandshakeChallengeRequest);

                /** WorkerHandshakeChallengeRequest requestId. */
                public requestId: string;

                /** WorkerHandshakeChallengeRequest traceId. */
                public traceId: string;

                /** WorkerHandshakeChallengeRequest workerId. */
                public workerId: string;

                /** WorkerHandshakeChallengeRequest proofKeyId. */
                public proofKeyId: string;

                /** WorkerHandshakeChallengeRequest proofAlgorithm. */
                public proofAlgorithm: cordum.agent.v1.WorkerHandshakeProofAlgorithm;

                /** WorkerHandshakeChallengeRequest audience. */
                public audience: string;

                /** WorkerHandshakeChallengeRequest purpose. */
                public purpose: cordum.agent.v1.WorkerHandshakePurpose;

                /** WorkerHandshakeChallengeRequest clientNonce. */
                public clientNonce: Uint8Array;

                /** WorkerHandshakeChallengeRequest protocolVersion. */
                public protocolVersion: number;

                /** WorkerHandshakeChallengeRequest sdkVersion. */
                public sdkVersion: string;

                /**
                 * Creates a new WorkerHandshakeChallengeRequest instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns WorkerHandshakeChallengeRequest instance
                 */
                public static create(properties?: cordum.agent.v1.IWorkerHandshakeChallengeRequest): cordum.agent.v1.WorkerHandshakeChallengeRequest;

                /**
                 * Encodes the specified WorkerHandshakeChallengeRequest message. Does not implicitly {@link cordum.agent.v1.WorkerHandshakeChallengeRequest.verify|verify} messages.
                 * @param message WorkerHandshakeChallengeRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IWorkerHandshakeChallengeRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified WorkerHandshakeChallengeRequest message, length delimited. Does not implicitly {@link cordum.agent.v1.WorkerHandshakeChallengeRequest.verify|verify} messages.
                 * @param message WorkerHandshakeChallengeRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IWorkerHandshakeChallengeRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a WorkerHandshakeChallengeRequest message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns WorkerHandshakeChallengeRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.WorkerHandshakeChallengeRequest;

                /**
                 * Decodes a WorkerHandshakeChallengeRequest message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns WorkerHandshakeChallengeRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.WorkerHandshakeChallengeRequest;

                /**
                 * Verifies a WorkerHandshakeChallengeRequest message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a WorkerHandshakeChallengeRequest message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns WorkerHandshakeChallengeRequest
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.WorkerHandshakeChallengeRequest;

                /**
                 * Creates a plain object from a WorkerHandshakeChallengeRequest message. Also converts values to other types if specified.
                 * @param message WorkerHandshakeChallengeRequest
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.WorkerHandshakeChallengeRequest, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this WorkerHandshakeChallengeRequest to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for WorkerHandshakeChallengeRequest
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a WorkerHandshakeChallenge. */
            interface IWorkerHandshakeChallenge {

                /** WorkerHandshakeChallenge requestId */
                requestId?: (string|null);

                /** WorkerHandshakeChallenge challengeId */
                challengeId?: (string|null);

                /** WorkerHandshakeChallenge traceId */
                traceId?: (string|null);

                /** WorkerHandshakeChallenge workerId */
                workerId?: (string|null);

                /** WorkerHandshakeChallenge agentId */
                agentId?: (string|null);

                /** WorkerHandshakeChallenge tenantId */
                tenantId?: (string|null);

                /** WorkerHandshakeChallenge proofKeyId */
                proofKeyId?: (string|null);

                /** WorkerHandshakeChallenge proofAlgorithm */
                proofAlgorithm?: (cordum.agent.v1.WorkerHandshakeProofAlgorithm|null);

                /** WorkerHandshakeChallenge serverKeyId */
                serverKeyId?: (string|null);

                /** WorkerHandshakeChallenge audience */
                audience?: (string|null);

                /** WorkerHandshakeChallenge purpose */
                purpose?: (cordum.agent.v1.WorkerHandshakePurpose|null);

                /** WorkerHandshakeChallenge clientNonce */
                clientNonce?: (Uint8Array|null);

                /** WorkerHandshakeChallenge serverNonce */
                serverNonce?: (Uint8Array|null);

                /** WorkerHandshakeChallenge protocolVersion */
                protocolVersion?: (number|null);

                /** WorkerHandshakeChallenge sdkVersion */
                sdkVersion?: (string|null);

                /** WorkerHandshakeChallenge issuedAt */
                issuedAt?: (google.protobuf.ITimestamp|null);

                /** WorkerHandshakeChallenge expiresAt */
                expiresAt?: (google.protobuf.ITimestamp|null);
            }

            /** Represents a WorkerHandshakeChallenge. */
            class WorkerHandshakeChallenge implements IWorkerHandshakeChallenge {

                /**
                 * Constructs a new WorkerHandshakeChallenge.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IWorkerHandshakeChallenge);

                /** WorkerHandshakeChallenge requestId. */
                public requestId: string;

                /** WorkerHandshakeChallenge challengeId. */
                public challengeId: string;

                /** WorkerHandshakeChallenge traceId. */
                public traceId: string;

                /** WorkerHandshakeChallenge workerId. */
                public workerId: string;

                /** WorkerHandshakeChallenge agentId. */
                public agentId: string;

                /** WorkerHandshakeChallenge tenantId. */
                public tenantId: string;

                /** WorkerHandshakeChallenge proofKeyId. */
                public proofKeyId: string;

                /** WorkerHandshakeChallenge proofAlgorithm. */
                public proofAlgorithm: cordum.agent.v1.WorkerHandshakeProofAlgorithm;

                /** WorkerHandshakeChallenge serverKeyId. */
                public serverKeyId: string;

                /** WorkerHandshakeChallenge audience. */
                public audience: string;

                /** WorkerHandshakeChallenge purpose. */
                public purpose: cordum.agent.v1.WorkerHandshakePurpose;

                /** WorkerHandshakeChallenge clientNonce. */
                public clientNonce: Uint8Array;

                /** WorkerHandshakeChallenge serverNonce. */
                public serverNonce: Uint8Array;

                /** WorkerHandshakeChallenge protocolVersion. */
                public protocolVersion: number;

                /** WorkerHandshakeChallenge sdkVersion. */
                public sdkVersion: string;

                /** WorkerHandshakeChallenge issuedAt. */
                public issuedAt?: (google.protobuf.ITimestamp|null);

                /** WorkerHandshakeChallenge expiresAt. */
                public expiresAt?: (google.protobuf.ITimestamp|null);

                /**
                 * Creates a new WorkerHandshakeChallenge instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns WorkerHandshakeChallenge instance
                 */
                public static create(properties?: cordum.agent.v1.IWorkerHandshakeChallenge): cordum.agent.v1.WorkerHandshakeChallenge;

                /**
                 * Encodes the specified WorkerHandshakeChallenge message. Does not implicitly {@link cordum.agent.v1.WorkerHandshakeChallenge.verify|verify} messages.
                 * @param message WorkerHandshakeChallenge message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IWorkerHandshakeChallenge, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified WorkerHandshakeChallenge message, length delimited. Does not implicitly {@link cordum.agent.v1.WorkerHandshakeChallenge.verify|verify} messages.
                 * @param message WorkerHandshakeChallenge message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IWorkerHandshakeChallenge, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a WorkerHandshakeChallenge message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns WorkerHandshakeChallenge
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.WorkerHandshakeChallenge;

                /**
                 * Decodes a WorkerHandshakeChallenge message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns WorkerHandshakeChallenge
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.WorkerHandshakeChallenge;

                /**
                 * Verifies a WorkerHandshakeChallenge message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a WorkerHandshakeChallenge message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns WorkerHandshakeChallenge
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.WorkerHandshakeChallenge;

                /**
                 * Creates a plain object from a WorkerHandshakeChallenge message. Also converts values to other types if specified.
                 * @param message WorkerHandshakeChallenge
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.WorkerHandshakeChallenge, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this WorkerHandshakeChallenge to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for WorkerHandshakeChallenge
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a WorkerHandshakeAuthenticate. */
            interface IWorkerHandshakeAuthenticate {

                /** WorkerHandshakeAuthenticate challenge */
                challenge?: (cordum.agent.v1.IWorkerHandshakeChallenge|null);

                /** WorkerHandshakeAuthenticate capabilityHandshake */
                capabilityHandshake?: (cordum.agent.v1.IHandshake|null);
            }

            /** Represents a WorkerHandshakeAuthenticate. */
            class WorkerHandshakeAuthenticate implements IWorkerHandshakeAuthenticate {

                /**
                 * Constructs a new WorkerHandshakeAuthenticate.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IWorkerHandshakeAuthenticate);

                /** WorkerHandshakeAuthenticate challenge. */
                public challenge?: (cordum.agent.v1.IWorkerHandshakeChallenge|null);

                /** WorkerHandshakeAuthenticate capabilityHandshake. */
                public capabilityHandshake?: (cordum.agent.v1.IHandshake|null);

                /**
                 * Creates a new WorkerHandshakeAuthenticate instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns WorkerHandshakeAuthenticate instance
                 */
                public static create(properties?: cordum.agent.v1.IWorkerHandshakeAuthenticate): cordum.agent.v1.WorkerHandshakeAuthenticate;

                /**
                 * Encodes the specified WorkerHandshakeAuthenticate message. Does not implicitly {@link cordum.agent.v1.WorkerHandshakeAuthenticate.verify|verify} messages.
                 * @param message WorkerHandshakeAuthenticate message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IWorkerHandshakeAuthenticate, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified WorkerHandshakeAuthenticate message, length delimited. Does not implicitly {@link cordum.agent.v1.WorkerHandshakeAuthenticate.verify|verify} messages.
                 * @param message WorkerHandshakeAuthenticate message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IWorkerHandshakeAuthenticate, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a WorkerHandshakeAuthenticate message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns WorkerHandshakeAuthenticate
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.WorkerHandshakeAuthenticate;

                /**
                 * Decodes a WorkerHandshakeAuthenticate message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns WorkerHandshakeAuthenticate
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.WorkerHandshakeAuthenticate;

                /**
                 * Verifies a WorkerHandshakeAuthenticate message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a WorkerHandshakeAuthenticate message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns WorkerHandshakeAuthenticate
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.WorkerHandshakeAuthenticate;

                /**
                 * Creates a plain object from a WorkerHandshakeAuthenticate message. Also converts values to other types if specified.
                 * @param message WorkerHandshakeAuthenticate
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.WorkerHandshakeAuthenticate, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this WorkerHandshakeAuthenticate to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for WorkerHandshakeAuthenticate
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a WorkerHandshakeResult. */
            interface IWorkerHandshakeResult {

                /** WorkerHandshakeResult challenge */
                challenge?: (cordum.agent.v1.IWorkerHandshakeChallenge|null);

                /** WorkerHandshakeResult accepted */
                accepted?: (boolean|null);

                /** WorkerHandshakeResult rejectionReason */
                rejectionReason?: (cordum.agent.v1.WorkerHandshakeRejectionReason|null);

                /** WorkerHandshakeResult tokenExpiresAt */
                tokenExpiresAt?: (google.protobuf.ITimestamp|null);

                /** WorkerHandshakeResult issuedAt */
                issuedAt?: (google.protobuf.ITimestamp|null);
            }

            /** Represents a WorkerHandshakeResult. */
            class WorkerHandshakeResult implements IWorkerHandshakeResult {

                /**
                 * Constructs a new WorkerHandshakeResult.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IWorkerHandshakeResult);

                /** WorkerHandshakeResult challenge. */
                public challenge?: (cordum.agent.v1.IWorkerHandshakeChallenge|null);

                /** WorkerHandshakeResult accepted. */
                public accepted: boolean;

                /** WorkerHandshakeResult rejectionReason. */
                public rejectionReason: cordum.agent.v1.WorkerHandshakeRejectionReason;

                /** WorkerHandshakeResult tokenExpiresAt. */
                public tokenExpiresAt?: (google.protobuf.ITimestamp|null);

                /** WorkerHandshakeResult issuedAt. */
                public issuedAt?: (google.protobuf.ITimestamp|null);

                /**
                 * Creates a new WorkerHandshakeResult instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns WorkerHandshakeResult instance
                 */
                public static create(properties?: cordum.agent.v1.IWorkerHandshakeResult): cordum.agent.v1.WorkerHandshakeResult;

                /**
                 * Encodes the specified WorkerHandshakeResult message. Does not implicitly {@link cordum.agent.v1.WorkerHandshakeResult.verify|verify} messages.
                 * @param message WorkerHandshakeResult message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IWorkerHandshakeResult, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified WorkerHandshakeResult message, length delimited. Does not implicitly {@link cordum.agent.v1.WorkerHandshakeResult.verify|verify} messages.
                 * @param message WorkerHandshakeResult message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IWorkerHandshakeResult, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a WorkerHandshakeResult message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns WorkerHandshakeResult
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.WorkerHandshakeResult;

                /**
                 * Decodes a WorkerHandshakeResult message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns WorkerHandshakeResult
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.WorkerHandshakeResult;

                /**
                 * Verifies a WorkerHandshakeResult message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a WorkerHandshakeResult message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns WorkerHandshakeResult
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.WorkerHandshakeResult;

                /**
                 * Creates a plain object from a WorkerHandshakeResult message. Also converts values to other types if specified.
                 * @param message WorkerHandshakeResult
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.WorkerHandshakeResult, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this WorkerHandshakeResult to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for WorkerHandshakeResult
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** RuleType enum. */
            enum RuleType {
                RULE_TYPE_UNSPECIFIED = 0,
                RULE_TYPE_INPUT = 1,
                RULE_TYPE_OUTPUT = 2,
                RULE_TYPE_VELOCITY = 3,
                RULE_TYPE_EDGE = 4
            }

            /** RuleStatus enum. */
            enum RuleStatus {
                RULE_STATUS_UNSPECIFIED = 0,
                RULE_STATUS_DRAFT = 1,
                RULE_STATUS_PUBLISHED = 2,
                RULE_STATUS_DEPRECATED = 3
            }

            /** DecisionSource enum. */
            enum DecisionSource {
                DECISION_SOURCE_UNSPECIFIED = 0,
                DECISION_SOURCE_JOB = 1,
                DECISION_SOURCE_EDGE = 2
            }

            /** RuleScopeKind enum. */
            enum RuleScopeKind {
                RULE_SCOPE_KIND_UNSPECIFIED = 0,
                RULE_SCOPE_KIND_GLOBAL = 1,
                RULE_SCOPE_KIND_TENANT = 2,
                RULE_SCOPE_KIND_WORKFLOW = 3,
                RULE_SCOPE_KIND_EDGE_FLEET = 4,
                RULE_SCOPE_KIND_EDGE_USER = 5
            }

            /** EdgeMode enum. */
            enum EdgeMode {
                EDGE_MODE_UNSPECIFIED = 0,
                EDGE_MODE_OBSERVE = 1,
                EDGE_MODE_ENFORCE = 2,
                EDGE_MODE_ENTERPRISE_STRICT = 3
            }

            /** Properties of a RuleScope. */
            interface IRuleScope {

                /** RuleScope kind */
                kind?: (cordum.agent.v1.RuleScopeKind|null);

                /** RuleScope value */
                value?: (string|null);
            }

            /** Represents a RuleScope. */
            class RuleScope implements IRuleScope {

                /**
                 * Constructs a new RuleScope.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IRuleScope);

                /** RuleScope kind. */
                public kind: cordum.agent.v1.RuleScopeKind;

                /** RuleScope value. */
                public value: string;

                /**
                 * Creates a new RuleScope instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns RuleScope instance
                 */
                public static create(properties?: cordum.agent.v1.IRuleScope): cordum.agent.v1.RuleScope;

                /**
                 * Encodes the specified RuleScope message. Does not implicitly {@link cordum.agent.v1.RuleScope.verify|verify} messages.
                 * @param message RuleScope message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IRuleScope, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified RuleScope message, length delimited. Does not implicitly {@link cordum.agent.v1.RuleScope.verify|verify} messages.
                 * @param message RuleScope message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IRuleScope, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a RuleScope message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns RuleScope
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.RuleScope;

                /**
                 * Decodes a RuleScope message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns RuleScope
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.RuleScope;

                /**
                 * Verifies a RuleScope message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a RuleScope message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns RuleScope
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.RuleScope;

                /**
                 * Creates a plain object from a RuleScope message. Also converts values to other types if specified.
                 * @param message RuleScope
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.RuleScope, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this RuleScope to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for RuleScope
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of an AuditMetadata. */
            interface IAuditMetadata {

                /** AuditMetadata createdAt */
                createdAt?: (google.protobuf.ITimestamp|null);

                /** AuditMetadata createdBy */
                createdBy?: (string|null);

                /** AuditMetadata updatedAt */
                updatedAt?: (google.protobuf.ITimestamp|null);

                /** AuditMetadata updatedBy */
                updatedBy?: (string|null);
            }

            /** Represents an AuditMetadata. */
            class AuditMetadata implements IAuditMetadata {

                /**
                 * Constructs a new AuditMetadata.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IAuditMetadata);

                /** AuditMetadata createdAt. */
                public createdAt?: (google.protobuf.ITimestamp|null);

                /** AuditMetadata createdBy. */
                public createdBy: string;

                /** AuditMetadata updatedAt. */
                public updatedAt?: (google.protobuf.ITimestamp|null);

                /** AuditMetadata updatedBy. */
                public updatedBy: string;

                /**
                 * Creates a new AuditMetadata instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns AuditMetadata instance
                 */
                public static create(properties?: cordum.agent.v1.IAuditMetadata): cordum.agent.v1.AuditMetadata;

                /**
                 * Encodes the specified AuditMetadata message. Does not implicitly {@link cordum.agent.v1.AuditMetadata.verify|verify} messages.
                 * @param message AuditMetadata message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IAuditMetadata, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified AuditMetadata message, length delimited. Does not implicitly {@link cordum.agent.v1.AuditMetadata.verify|verify} messages.
                 * @param message AuditMetadata message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IAuditMetadata, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes an AuditMetadata message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns AuditMetadata
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.AuditMetadata;

                /**
                 * Decodes an AuditMetadata message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns AuditMetadata
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.AuditMetadata;

                /**
                 * Verifies an AuditMetadata message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates an AuditMetadata message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns AuditMetadata
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.AuditMetadata;

                /**
                 * Creates a plain object from an AuditMetadata message. Also converts values to other types if specified.
                 * @param message AuditMetadata
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.AuditMetadata, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this AuditMetadata to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for AuditMetadata
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a Rule. */
            interface IRule {

                /** Rule id */
                id?: (string|null);

                /** Rule name */
                name?: (string|null);

                /** Rule type */
                type?: (cordum.agent.v1.RuleType|null);

                /** Rule scope */
                scope?: (cordum.agent.v1.IRuleScope|null);

                /** Rule status */
                status?: (cordum.agent.v1.RuleStatus|null);

                /** Rule version */
                version?: (string|null);

                /** Rule audit */
                audit?: (cordum.agent.v1.IAuditMetadata|null);

                /** Rule match */
                match?: (google.protobuf.IStruct|null);

                /** Rule decide */
                decide?: (google.protobuf.IStruct|null);

                /** Rule description */
                description?: (string|null);
            }

            /** Represents a Rule. */
            class Rule implements IRule {

                /**
                 * Constructs a new Rule.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IRule);

                /** Rule id. */
                public id: string;

                /** Rule name. */
                public name: string;

                /** Rule type. */
                public type: cordum.agent.v1.RuleType;

                /** Rule scope. */
                public scope?: (cordum.agent.v1.IRuleScope|null);

                /** Rule status. */
                public status: cordum.agent.v1.RuleStatus;

                /** Rule version. */
                public version: string;

                /** Rule audit. */
                public audit?: (cordum.agent.v1.IAuditMetadata|null);

                /** Rule match. */
                public match?: (google.protobuf.IStruct|null);

                /** Rule decide. */
                public decide?: (google.protobuf.IStruct|null);

                /** Rule description. */
                public description: string;

                /**
                 * Creates a new Rule instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns Rule instance
                 */
                public static create(properties?: cordum.agent.v1.IRule): cordum.agent.v1.Rule;

                /**
                 * Encodes the specified Rule message. Does not implicitly {@link cordum.agent.v1.Rule.verify|verify} messages.
                 * @param message Rule message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IRule, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified Rule message, length delimited. Does not implicitly {@link cordum.agent.v1.Rule.verify|verify} messages.
                 * @param message Rule message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IRule, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a Rule message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns Rule
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.Rule;

                /**
                 * Decodes a Rule message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns Rule
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.Rule;

                /**
                 * Verifies a Rule message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a Rule message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns Rule
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.Rule;

                /**
                 * Creates a plain object from a Rule message. Also converts values to other types if specified.
                 * @param message Rule
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.Rule, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this Rule to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for Rule
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a TraceStep. */
            interface ITraceStep {

                /** TraceStep ruleId */
                ruleId?: (string|null);

                /** TraceStep bundleId */
                bundleId?: (string|null);

                /** TraceStep decisionType */
                decisionType?: (cordum.agent.v1.DecisionType|null);

                /** TraceStep reason */
                reason?: (string|null);

                /** TraceStep timestamp */
                timestamp?: (google.protobuf.ITimestamp|null);

                /** TraceStep constraints */
                constraints?: (google.protobuf.IStruct|null);
            }

            /** Represents a TraceStep. */
            class TraceStep implements ITraceStep {

                /**
                 * Constructs a new TraceStep.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.ITraceStep);

                /** TraceStep ruleId. */
                public ruleId: string;

                /** TraceStep bundleId. */
                public bundleId: string;

                /** TraceStep decisionType. */
                public decisionType: cordum.agent.v1.DecisionType;

                /** TraceStep reason. */
                public reason: string;

                /** TraceStep timestamp. */
                public timestamp?: (google.protobuf.ITimestamp|null);

                /** TraceStep constraints. */
                public constraints?: (google.protobuf.IStruct|null);

                /**
                 * Creates a new TraceStep instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns TraceStep instance
                 */
                public static create(properties?: cordum.agent.v1.ITraceStep): cordum.agent.v1.TraceStep;

                /**
                 * Encodes the specified TraceStep message. Does not implicitly {@link cordum.agent.v1.TraceStep.verify|verify} messages.
                 * @param message TraceStep message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.ITraceStep, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified TraceStep message, length delimited. Does not implicitly {@link cordum.agent.v1.TraceStep.verify|verify} messages.
                 * @param message TraceStep message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.ITraceStep, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a TraceStep message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns TraceStep
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.TraceStep;

                /**
                 * Decodes a TraceStep message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns TraceStep
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.TraceStep;

                /**
                 * Verifies a TraceStep message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a TraceStep message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns TraceStep
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.TraceStep;

                /**
                 * Creates a plain object from a TraceStep message. Also converts values to other types if specified.
                 * @param message TraceStep
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.TraceStep, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this TraceStep to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for TraceStep
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a Decision. */
            interface IDecision {

                /** Decision source */
                source?: (cordum.agent.v1.DecisionSource|null);

                /** Decision ruleId */
                ruleId?: (string|null);

                /** Decision bundleId */
                bundleId?: (string|null);

                /** Decision bundleVersion */
                bundleVersion?: (string|null);

                /** Decision type */
                type?: (cordum.agent.v1.DecisionType|null);

                /** Decision trace */
                trace?: (cordum.agent.v1.ITraceStep[]|null);

                /** Decision inputRef */
                inputRef?: (string|null);

                /** Decision outputRef */
                outputRef?: (string|null);

                /** Decision auditHash */
                auditHash?: (string|null);

                /** Decision timestamp */
                timestamp?: (google.protobuf.ITimestamp|null);
            }

            /** Represents a Decision. */
            class Decision implements IDecision {

                /**
                 * Constructs a new Decision.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IDecision);

                /** Decision source. */
                public source: cordum.agent.v1.DecisionSource;

                /** Decision ruleId. */
                public ruleId: string;

                /** Decision bundleId. */
                public bundleId: string;

                /** Decision bundleVersion. */
                public bundleVersion: string;

                /** Decision type. */
                public type: cordum.agent.v1.DecisionType;

                /** Decision trace. */
                public trace: cordum.agent.v1.ITraceStep[];

                /** Decision inputRef. */
                public inputRef: string;

                /** Decision outputRef. */
                public outputRef: string;

                /** Decision auditHash. */
                public auditHash: string;

                /** Decision timestamp. */
                public timestamp?: (google.protobuf.ITimestamp|null);

                /**
                 * Creates a new Decision instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns Decision instance
                 */
                public static create(properties?: cordum.agent.v1.IDecision): cordum.agent.v1.Decision;

                /**
                 * Encodes the specified Decision message. Does not implicitly {@link cordum.agent.v1.Decision.verify|verify} messages.
                 * @param message Decision message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IDecision, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified Decision message, length delimited. Does not implicitly {@link cordum.agent.v1.Decision.verify|verify} messages.
                 * @param message Decision message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IDecision, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a Decision message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns Decision
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.Decision;

                /**
                 * Decodes a Decision message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns Decision
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.Decision;

                /**
                 * Verifies a Decision message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a Decision message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns Decision
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.Decision;

                /**
                 * Creates a plain object from a Decision message. Also converts values to other types if specified.
                 * @param message Decision
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.Decision, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this Decision to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for Decision
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a BundleMetadata. */
            interface IBundleMetadata {

                /** BundleMetadata edgeMode */
                edgeMode?: (cordum.agent.v1.EdgeMode|null);
            }

            /** Represents a BundleMetadata. */
            class BundleMetadata implements IBundleMetadata {

                /**
                 * Constructs a new BundleMetadata.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IBundleMetadata);

                /** BundleMetadata edgeMode. */
                public edgeMode: cordum.agent.v1.EdgeMode;

                /**
                 * Creates a new BundleMetadata instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns BundleMetadata instance
                 */
                public static create(properties?: cordum.agent.v1.IBundleMetadata): cordum.agent.v1.BundleMetadata;

                /**
                 * Encodes the specified BundleMetadata message. Does not implicitly {@link cordum.agent.v1.BundleMetadata.verify|verify} messages.
                 * @param message BundleMetadata message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IBundleMetadata, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified BundleMetadata message, length delimited. Does not implicitly {@link cordum.agent.v1.BundleMetadata.verify|verify} messages.
                 * @param message BundleMetadata message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IBundleMetadata, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a BundleMetadata message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns BundleMetadata
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.BundleMetadata;

                /**
                 * Decodes a BundleMetadata message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns BundleMetadata
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.BundleMetadata;

                /**
                 * Verifies a BundleMetadata message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a BundleMetadata message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns BundleMetadata
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.BundleMetadata;

                /**
                 * Creates a plain object from a BundleMetadata message. Also converts values to other types if specified.
                 * @param message BundleMetadata
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.BundleMetadata, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this BundleMetadata to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for BundleMetadata
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a BundleVersion. */
            interface IBundleVersion {

                /** BundleVersion version */
                version?: (string|null);

                /** BundleVersion ruleSnapshot */
                ruleSnapshot?: (cordum.agent.v1.IRule[]|null);

                /** BundleVersion deployedAt */
                deployedAt?: (google.protobuf.ITimestamp|null);

                /** BundleVersion auditHash */
                auditHash?: (string|null);
            }

            /** Represents a BundleVersion. */
            class BundleVersion implements IBundleVersion {

                /**
                 * Constructs a new BundleVersion.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IBundleVersion);

                /** BundleVersion version. */
                public version: string;

                /** BundleVersion ruleSnapshot. */
                public ruleSnapshot: cordum.agent.v1.IRule[];

                /** BundleVersion deployedAt. */
                public deployedAt?: (google.protobuf.ITimestamp|null);

                /** BundleVersion auditHash. */
                public auditHash: string;

                /**
                 * Creates a new BundleVersion instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns BundleVersion instance
                 */
                public static create(properties?: cordum.agent.v1.IBundleVersion): cordum.agent.v1.BundleVersion;

                /**
                 * Encodes the specified BundleVersion message. Does not implicitly {@link cordum.agent.v1.BundleVersion.verify|verify} messages.
                 * @param message BundleVersion message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IBundleVersion, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified BundleVersion message, length delimited. Does not implicitly {@link cordum.agent.v1.BundleVersion.verify|verify} messages.
                 * @param message BundleVersion message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IBundleVersion, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a BundleVersion message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns BundleVersion
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.BundleVersion;

                /**
                 * Decodes a BundleVersion message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns BundleVersion
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.BundleVersion;

                /**
                 * Verifies a BundleVersion message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a BundleVersion message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns BundleVersion
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.BundleVersion;

                /**
                 * Creates a plain object from a BundleVersion message. Also converts values to other types if specified.
                 * @param message BundleVersion
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.BundleVersion, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this BundleVersion to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for BundleVersion
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a Bundle. */
            interface IBundle {

                /** Bundle id */
                id?: (string|null);

                /** Bundle name */
                name?: (string|null);

                /** Bundle ruleIds */
                ruleIds?: (string[]|null);

                /** Bundle scopeBinding */
                scopeBinding?: (cordum.agent.v1.IRuleScope|null);

                /** Bundle versions */
                versions?: (cordum.agent.v1.IBundleVersion[]|null);

                /** Bundle metadata */
                metadata?: (cordum.agent.v1.IBundleMetadata|null);
            }

            /** Represents a Bundle. */
            class Bundle implements IBundle {

                /**
                 * Constructs a new Bundle.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IBundle);

                /** Bundle id. */
                public id: string;

                /** Bundle name. */
                public name: string;

                /** Bundle ruleIds. */
                public ruleIds: string[];

                /** Bundle scopeBinding. */
                public scopeBinding?: (cordum.agent.v1.IRuleScope|null);

                /** Bundle versions. */
                public versions: cordum.agent.v1.IBundleVersion[];

                /** Bundle metadata. */
                public metadata?: (cordum.agent.v1.IBundleMetadata|null);

                /**
                 * Creates a new Bundle instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns Bundle instance
                 */
                public static create(properties?: cordum.agent.v1.IBundle): cordum.agent.v1.Bundle;

                /**
                 * Encodes the specified Bundle message. Does not implicitly {@link cordum.agent.v1.Bundle.verify|verify} messages.
                 * @param message Bundle message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IBundle, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified Bundle message, length delimited. Does not implicitly {@link cordum.agent.v1.Bundle.verify|verify} messages.
                 * @param message Bundle message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IBundle, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a Bundle message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns Bundle
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.Bundle;

                /**
                 * Decodes a Bundle message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns Bundle
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.Bundle;

                /**
                 * Verifies a Bundle message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a Bundle message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns Bundle
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.Bundle;

                /**
                 * Creates a plain object from a Bundle message. Also converts values to other types if specified.
                 * @param message Bundle
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.Bundle, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this Bundle to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for Bundle
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a JobEvaluationContext. */
            interface IJobEvaluationContext {

                /** JobEvaluationContext tenantId */
                tenantId?: (string|null);

                /** JobEvaluationContext jobId */
                jobId?: (string|null);

                /** JobEvaluationContext workflowId */
                workflowId?: (string|null);

                /** JobEvaluationContext topic */
                topic?: (string|null);

                /** JobEvaluationContext principalId */
                principalId?: (string|null);

                /** JobEvaluationContext labels */
                labels?: ({ [k: string]: string }|null);

                /** JobEvaluationContext memoryId */
                memoryId?: (string|null);

                /** JobEvaluationContext capability */
                capability?: (string|null);

                /** JobEvaluationContext riskTags */
                riskTags?: (string[]|null);

                /** JobEvaluationContext inputContent */
                inputContent?: (Uint8Array|null);

                /** JobEvaluationContext inputContentType */
                inputContentType?: (string|null);

                /** JobEvaluationContext inputSizeBytes */
                inputSizeBytes?: (number|Long|null);
            }

            /** Represents a JobEvaluationContext. */
            class JobEvaluationContext implements IJobEvaluationContext {

                /**
                 * Constructs a new JobEvaluationContext.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IJobEvaluationContext);

                /** JobEvaluationContext tenantId. */
                public tenantId: string;

                /** JobEvaluationContext jobId. */
                public jobId: string;

                /** JobEvaluationContext workflowId. */
                public workflowId: string;

                /** JobEvaluationContext topic. */
                public topic: string;

                /** JobEvaluationContext principalId. */
                public principalId: string;

                /** JobEvaluationContext labels. */
                public labels: { [k: string]: string };

                /** JobEvaluationContext memoryId. */
                public memoryId: string;

                /** JobEvaluationContext capability. */
                public capability: string;

                /** JobEvaluationContext riskTags. */
                public riskTags: string[];

                /** JobEvaluationContext inputContent. */
                public inputContent: Uint8Array;

                /** JobEvaluationContext inputContentType. */
                public inputContentType: string;

                /** JobEvaluationContext inputSizeBytes. */
                public inputSizeBytes: (number|Long);

                /**
                 * Creates a new JobEvaluationContext instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns JobEvaluationContext instance
                 */
                public static create(properties?: cordum.agent.v1.IJobEvaluationContext): cordum.agent.v1.JobEvaluationContext;

                /**
                 * Encodes the specified JobEvaluationContext message. Does not implicitly {@link cordum.agent.v1.JobEvaluationContext.verify|verify} messages.
                 * @param message JobEvaluationContext message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IJobEvaluationContext, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified JobEvaluationContext message, length delimited. Does not implicitly {@link cordum.agent.v1.JobEvaluationContext.verify|verify} messages.
                 * @param message JobEvaluationContext message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IJobEvaluationContext, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a JobEvaluationContext message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns JobEvaluationContext
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.JobEvaluationContext;

                /**
                 * Decodes a JobEvaluationContext message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns JobEvaluationContext
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.JobEvaluationContext;

                /**
                 * Verifies a JobEvaluationContext message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a JobEvaluationContext message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns JobEvaluationContext
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.JobEvaluationContext;

                /**
                 * Creates a plain object from a JobEvaluationContext message. Also converts values to other types if specified.
                 * @param message JobEvaluationContext
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.JobEvaluationContext, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this JobEvaluationContext to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for JobEvaluationContext
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of an EdgeEvaluationContext. */
            interface IEdgeEvaluationContext {

                /** EdgeEvaluationContext tenantId */
                tenantId?: (string|null);

                /** EdgeEvaluationContext principalId */
                principalId?: (string|null);

                /** EdgeEvaluationContext sessionId */
                sessionId?: (string|null);

                /** EdgeEvaluationContext executionId */
                executionId?: (string|null);

                /** EdgeEvaluationContext agentProduct */
                agentProduct?: (string|null);

                /** EdgeEvaluationContext toolName */
                toolName?: (string|null);

                /** EdgeEvaluationContext toolInputRedacted */
                toolInputRedacted?: (google.protobuf.IStruct|null);

                /** EdgeEvaluationContext inputHash */
                inputHash?: (string|null);

                /** EdgeEvaluationContext toolInputHash */
                toolInputHash?: (string|null);

                /** EdgeEvaluationContext labels */
                labels?: ({ [k: string]: string }|null);

                /** EdgeEvaluationContext riskTags */
                riskTags?: (string[]|null);
            }

            /** Represents an EdgeEvaluationContext. */
            class EdgeEvaluationContext implements IEdgeEvaluationContext {

                /**
                 * Constructs a new EdgeEvaluationContext.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IEdgeEvaluationContext);

                /** EdgeEvaluationContext tenantId. */
                public tenantId: string;

                /** EdgeEvaluationContext principalId. */
                public principalId: string;

                /** EdgeEvaluationContext sessionId. */
                public sessionId: string;

                /** EdgeEvaluationContext executionId. */
                public executionId: string;

                /** EdgeEvaluationContext agentProduct. */
                public agentProduct: string;

                /** EdgeEvaluationContext toolName. */
                public toolName: string;

                /** EdgeEvaluationContext toolInputRedacted. */
                public toolInputRedacted?: (google.protobuf.IStruct|null);

                /** EdgeEvaluationContext inputHash. */
                public inputHash: string;

                /** EdgeEvaluationContext toolInputHash. */
                public toolInputHash: string;

                /** EdgeEvaluationContext labels. */
                public labels: { [k: string]: string };

                /** EdgeEvaluationContext riskTags. */
                public riskTags: string[];

                /**
                 * Creates a new EdgeEvaluationContext instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns EdgeEvaluationContext instance
                 */
                public static create(properties?: cordum.agent.v1.IEdgeEvaluationContext): cordum.agent.v1.EdgeEvaluationContext;

                /**
                 * Encodes the specified EdgeEvaluationContext message. Does not implicitly {@link cordum.agent.v1.EdgeEvaluationContext.verify|verify} messages.
                 * @param message EdgeEvaluationContext message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IEdgeEvaluationContext, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified EdgeEvaluationContext message, length delimited. Does not implicitly {@link cordum.agent.v1.EdgeEvaluationContext.verify|verify} messages.
                 * @param message EdgeEvaluationContext message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IEdgeEvaluationContext, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes an EdgeEvaluationContext message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns EdgeEvaluationContext
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.EdgeEvaluationContext;

                /**
                 * Decodes an EdgeEvaluationContext message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns EdgeEvaluationContext
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.EdgeEvaluationContext;

                /**
                 * Verifies an EdgeEvaluationContext message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates an EdgeEvaluationContext message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns EdgeEvaluationContext
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.EdgeEvaluationContext;

                /**
                 * Creates a plain object from an EdgeEvaluationContext message. Also converts values to other types if specified.
                 * @param message EdgeEvaluationContext
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.EdgeEvaluationContext, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this EdgeEvaluationContext to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for EdgeEvaluationContext
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a PolicyEvaluateRequest. */
            interface IPolicyEvaluateRequest {

                /** PolicyEvaluateRequest rule */
                rule?: (cordum.agent.v1.IRule|null);

                /** PolicyEvaluateRequest bundleId */
                bundleId?: (string|null);

                /** PolicyEvaluateRequest scope */
                scope?: (cordum.agent.v1.IRuleScope|null);

                /** PolicyEvaluateRequest jobContext */
                jobContext?: (cordum.agent.v1.IJobEvaluationContext|null);

                /** PolicyEvaluateRequest edgeContext */
                edgeContext?: (cordum.agent.v1.IEdgeEvaluationContext|null);
            }

            /** Represents a PolicyEvaluateRequest. */
            class PolicyEvaluateRequest implements IPolicyEvaluateRequest {

                /**
                 * Constructs a new PolicyEvaluateRequest.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IPolicyEvaluateRequest);

                /** PolicyEvaluateRequest rule. */
                public rule?: (cordum.agent.v1.IRule|null);

                /** PolicyEvaluateRequest bundleId. */
                public bundleId: string;

                /** PolicyEvaluateRequest scope. */
                public scope?: (cordum.agent.v1.IRuleScope|null);

                /** PolicyEvaluateRequest jobContext. */
                public jobContext?: (cordum.agent.v1.IJobEvaluationContext|null);

                /** PolicyEvaluateRequest edgeContext. */
                public edgeContext?: (cordum.agent.v1.IEdgeEvaluationContext|null);

                /**
                 * Creates a new PolicyEvaluateRequest instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns PolicyEvaluateRequest instance
                 */
                public static create(properties?: cordum.agent.v1.IPolicyEvaluateRequest): cordum.agent.v1.PolicyEvaluateRequest;

                /**
                 * Encodes the specified PolicyEvaluateRequest message. Does not implicitly {@link cordum.agent.v1.PolicyEvaluateRequest.verify|verify} messages.
                 * @param message PolicyEvaluateRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IPolicyEvaluateRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified PolicyEvaluateRequest message, length delimited. Does not implicitly {@link cordum.agent.v1.PolicyEvaluateRequest.verify|verify} messages.
                 * @param message PolicyEvaluateRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IPolicyEvaluateRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a PolicyEvaluateRequest message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns PolicyEvaluateRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.PolicyEvaluateRequest;

                /**
                 * Decodes a PolicyEvaluateRequest message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns PolicyEvaluateRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.PolicyEvaluateRequest;

                /**
                 * Verifies a PolicyEvaluateRequest message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a PolicyEvaluateRequest message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns PolicyEvaluateRequest
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.PolicyEvaluateRequest;

                /**
                 * Creates a plain object from a PolicyEvaluateRequest message. Also converts values to other types if specified.
                 * @param message PolicyEvaluateRequest
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.PolicyEvaluateRequest, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this PolicyEvaluateRequest to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for PolicyEvaluateRequest
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a PolicyEvaluateResponse. */
            interface IPolicyEvaluateResponse {

                /** PolicyEvaluateResponse decision */
                decision?: (cordum.agent.v1.IDecision|null);
            }

            /** Represents a PolicyEvaluateResponse. */
            class PolicyEvaluateResponse implements IPolicyEvaluateResponse {

                /**
                 * Constructs a new PolicyEvaluateResponse.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IPolicyEvaluateResponse);

                /** PolicyEvaluateResponse decision. */
                public decision?: (cordum.agent.v1.IDecision|null);

                /**
                 * Creates a new PolicyEvaluateResponse instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns PolicyEvaluateResponse instance
                 */
                public static create(properties?: cordum.agent.v1.IPolicyEvaluateResponse): cordum.agent.v1.PolicyEvaluateResponse;

                /**
                 * Encodes the specified PolicyEvaluateResponse message. Does not implicitly {@link cordum.agent.v1.PolicyEvaluateResponse.verify|verify} messages.
                 * @param message PolicyEvaluateResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IPolicyEvaluateResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified PolicyEvaluateResponse message, length delimited. Does not implicitly {@link cordum.agent.v1.PolicyEvaluateResponse.verify|verify} messages.
                 * @param message PolicyEvaluateResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IPolicyEvaluateResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a PolicyEvaluateResponse message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns PolicyEvaluateResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.PolicyEvaluateResponse;

                /**
                 * Decodes a PolicyEvaluateResponse message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns PolicyEvaluateResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.PolicyEvaluateResponse;

                /**
                 * Verifies a PolicyEvaluateResponse message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a PolicyEvaluateResponse message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns PolicyEvaluateResponse
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.PolicyEvaluateResponse;

                /**
                 * Creates a plain object from a PolicyEvaluateResponse message. Also converts values to other types if specified.
                 * @param message PolicyEvaluateResponse
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.PolicyEvaluateResponse, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this PolicyEvaluateResponse to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for PolicyEvaluateResponse
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Represents a PolicyEvaluator */
            class PolicyEvaluator extends $protobuf.rpc.Service {

                /**
                 * Constructs a new PolicyEvaluator service.
                 * @param rpcImpl RPC implementation
                 * @param [requestDelimited=false] Whether requests are length-delimited
                 * @param [responseDelimited=false] Whether responses are length-delimited
                 */
                constructor(rpcImpl: $protobuf.RPCImpl, requestDelimited?: boolean, responseDelimited?: boolean);

                /**
                 * Creates new PolicyEvaluator service using the specified rpc implementation.
                 * @param rpcImpl RPC implementation
                 * @param [requestDelimited=false] Whether requests are length-delimited
                 * @param [responseDelimited=false] Whether responses are length-delimited
                 * @returns RPC service. Useful where requests and/or responses are streamed.
                 */
                public static create(rpcImpl: $protobuf.RPCImpl, requestDelimited?: boolean, responseDelimited?: boolean): PolicyEvaluator;

                /**
                 * Calls EvaluateUnified.
                 * @param request PolicyEvaluateRequest message or plain object
                 * @param callback Node-style callback called with the error, if any, and PolicyEvaluateResponse
                 */
                public evaluateUnified(request: cordum.agent.v1.IPolicyEvaluateRequest, callback: cordum.agent.v1.PolicyEvaluator.EvaluateUnifiedCallback): void;

                /**
                 * Calls EvaluateUnified.
                 * @param request PolicyEvaluateRequest message or plain object
                 * @returns Promise
                 */
                public evaluateUnified(request: cordum.agent.v1.IPolicyEvaluateRequest): Promise<cordum.agent.v1.PolicyEvaluateResponse>;
            }

            namespace PolicyEvaluator {

                /**
                 * Callback as used by {@link cordum.agent.v1.PolicyEvaluator#evaluateUnified}.
                 * @param error Error, if any
                 * @param [response] PolicyEvaluateResponse
                 */
                type EvaluateUnifiedCallback = (error: (Error|null), response?: cordum.agent.v1.PolicyEvaluateResponse) => void;
            }

            /** DecisionType enum. */
            enum DecisionType {
                DECISION_TYPE_UNSPECIFIED = 0,
                DECISION_TYPE_ALLOW = 1,
                DECISION_TYPE_DENY = 2,
                DECISION_TYPE_REQUIRE_HUMAN = 3,
                DECISION_TYPE_THROTTLE = 4,
                DECISION_TYPE_ALLOW_WITH_CONSTRAINTS = 5,
                DECISION_TYPE_QUARANTINE = 6,
                DECISION_TYPE_REDACT = 7
            }

            /** Properties of a PolicyCheckRequest. */
            interface IPolicyCheckRequest {

                /** PolicyCheckRequest jobId */
                jobId?: (string|null);

                /** PolicyCheckRequest topic */
                topic?: (string|null);

                /** PolicyCheckRequest tenant */
                tenant?: (string|null);

                /** PolicyCheckRequest priority */
                priority?: (cordum.agent.v1.JobPriority|null);

                /** PolicyCheckRequest estimatedCost */
                estimatedCost?: (number|null);

                /** PolicyCheckRequest budget */
                budget?: (cordum.agent.v1.IBudget|null);

                /** PolicyCheckRequest principalId */
                principalId?: (string|null);

                /** PolicyCheckRequest labels */
                labels?: ({ [k: string]: string }|null);

                /** PolicyCheckRequest memoryId */
                memoryId?: (string|null);

                /** PolicyCheckRequest effectiveConfig */
                effectiveConfig?: (Uint8Array|null);

                /** PolicyCheckRequest meta */
                meta?: (cordum.agent.v1.IJobMetadata|null);

                /** PolicyCheckRequest inputContent */
                inputContent?: (Uint8Array|null);

                /** PolicyCheckRequest inputContentType */
                inputContentType?: (string|null);

                /** PolicyCheckRequest inputSizeBytes */
                inputSizeBytes?: (number|Long|null);
            }

            /** Represents a PolicyCheckRequest. */
            class PolicyCheckRequest implements IPolicyCheckRequest {

                /**
                 * Constructs a new PolicyCheckRequest.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IPolicyCheckRequest);

                /** PolicyCheckRequest jobId. */
                public jobId: string;

                /** PolicyCheckRequest topic. */
                public topic: string;

                /** PolicyCheckRequest tenant. */
                public tenant: string;

                /** PolicyCheckRequest priority. */
                public priority: cordum.agent.v1.JobPriority;

                /** PolicyCheckRequest estimatedCost. */
                public estimatedCost: number;

                /** PolicyCheckRequest budget. */
                public budget?: (cordum.agent.v1.IBudget|null);

                /** PolicyCheckRequest principalId. */
                public principalId: string;

                /** PolicyCheckRequest labels. */
                public labels: { [k: string]: string };

                /** PolicyCheckRequest memoryId. */
                public memoryId: string;

                /** PolicyCheckRequest effectiveConfig. */
                public effectiveConfig: Uint8Array;

                /** PolicyCheckRequest meta. */
                public meta?: (cordum.agent.v1.IJobMetadata|null);

                /** PolicyCheckRequest inputContent. */
                public inputContent: Uint8Array;

                /** PolicyCheckRequest inputContentType. */
                public inputContentType: string;

                /** PolicyCheckRequest inputSizeBytes. */
                public inputSizeBytes: (number|Long);

                /**
                 * Creates a new PolicyCheckRequest instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns PolicyCheckRequest instance
                 */
                public static create(properties?: cordum.agent.v1.IPolicyCheckRequest): cordum.agent.v1.PolicyCheckRequest;

                /**
                 * Encodes the specified PolicyCheckRequest message. Does not implicitly {@link cordum.agent.v1.PolicyCheckRequest.verify|verify} messages.
                 * @param message PolicyCheckRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IPolicyCheckRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified PolicyCheckRequest message, length delimited. Does not implicitly {@link cordum.agent.v1.PolicyCheckRequest.verify|verify} messages.
                 * @param message PolicyCheckRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IPolicyCheckRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a PolicyCheckRequest message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns PolicyCheckRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.PolicyCheckRequest;

                /**
                 * Decodes a PolicyCheckRequest message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns PolicyCheckRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.PolicyCheckRequest;

                /**
                 * Verifies a PolicyCheckRequest message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a PolicyCheckRequest message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns PolicyCheckRequest
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.PolicyCheckRequest;

                /**
                 * Creates a plain object from a PolicyCheckRequest message. Also converts values to other types if specified.
                 * @param message PolicyCheckRequest
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.PolicyCheckRequest, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this PolicyCheckRequest to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for PolicyCheckRequest
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a BudgetConstraints. */
            interface IBudgetConstraints {

                /** BudgetConstraints maxRuntimeMs */
                maxRuntimeMs?: (number|Long|null);

                /** BudgetConstraints maxRetries */
                maxRetries?: (number|null);

                /** BudgetConstraints maxArtifactBytes */
                maxArtifactBytes?: (number|Long|null);

                /** BudgetConstraints maxConcurrentJobs */
                maxConcurrentJobs?: (number|null);
            }

            /** Represents a BudgetConstraints. */
            class BudgetConstraints implements IBudgetConstraints {

                /**
                 * Constructs a new BudgetConstraints.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IBudgetConstraints);

                /** BudgetConstraints maxRuntimeMs. */
                public maxRuntimeMs: (number|Long);

                /** BudgetConstraints maxRetries. */
                public maxRetries: number;

                /** BudgetConstraints maxArtifactBytes. */
                public maxArtifactBytes: (number|Long);

                /** BudgetConstraints maxConcurrentJobs. */
                public maxConcurrentJobs: number;

                /**
                 * Creates a new BudgetConstraints instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns BudgetConstraints instance
                 */
                public static create(properties?: cordum.agent.v1.IBudgetConstraints): cordum.agent.v1.BudgetConstraints;

                /**
                 * Encodes the specified BudgetConstraints message. Does not implicitly {@link cordum.agent.v1.BudgetConstraints.verify|verify} messages.
                 * @param message BudgetConstraints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IBudgetConstraints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified BudgetConstraints message, length delimited. Does not implicitly {@link cordum.agent.v1.BudgetConstraints.verify|verify} messages.
                 * @param message BudgetConstraints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IBudgetConstraints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a BudgetConstraints message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns BudgetConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.BudgetConstraints;

                /**
                 * Decodes a BudgetConstraints message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns BudgetConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.BudgetConstraints;

                /**
                 * Verifies a BudgetConstraints message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a BudgetConstraints message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns BudgetConstraints
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.BudgetConstraints;

                /**
                 * Creates a plain object from a BudgetConstraints message. Also converts values to other types if specified.
                 * @param message BudgetConstraints
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.BudgetConstraints, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this BudgetConstraints to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for BudgetConstraints
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a SandboxProfile. */
            interface ISandboxProfile {

                /** SandboxProfile isolated */
                isolated?: (boolean|null);

                /** SandboxProfile networkAllowlist */
                networkAllowlist?: (string[]|null);

                /** SandboxProfile fsReadOnly */
                fsReadOnly?: (string[]|null);

                /** SandboxProfile fsReadWrite */
                fsReadWrite?: (string[]|null);
            }

            /** Represents a SandboxProfile. */
            class SandboxProfile implements ISandboxProfile {

                /**
                 * Constructs a new SandboxProfile.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.ISandboxProfile);

                /** SandboxProfile isolated. */
                public isolated: boolean;

                /** SandboxProfile networkAllowlist. */
                public networkAllowlist: string[];

                /** SandboxProfile fsReadOnly. */
                public fsReadOnly: string[];

                /** SandboxProfile fsReadWrite. */
                public fsReadWrite: string[];

                /**
                 * Creates a new SandboxProfile instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns SandboxProfile instance
                 */
                public static create(properties?: cordum.agent.v1.ISandboxProfile): cordum.agent.v1.SandboxProfile;

                /**
                 * Encodes the specified SandboxProfile message. Does not implicitly {@link cordum.agent.v1.SandboxProfile.verify|verify} messages.
                 * @param message SandboxProfile message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.ISandboxProfile, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified SandboxProfile message, length delimited. Does not implicitly {@link cordum.agent.v1.SandboxProfile.verify|verify} messages.
                 * @param message SandboxProfile message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.ISandboxProfile, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a SandboxProfile message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns SandboxProfile
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.SandboxProfile;

                /**
                 * Decodes a SandboxProfile message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns SandboxProfile
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.SandboxProfile;

                /**
                 * Verifies a SandboxProfile message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a SandboxProfile message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns SandboxProfile
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.SandboxProfile;

                /**
                 * Creates a plain object from a SandboxProfile message. Also converts values to other types if specified.
                 * @param message SandboxProfile
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.SandboxProfile, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this SandboxProfile to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for SandboxProfile
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a ToolchainConstraints. */
            interface IToolchainConstraints {

                /** ToolchainConstraints allowedTools */
                allowedTools?: (string[]|null);

                /** ToolchainConstraints allowedCommands */
                allowedCommands?: (string[]|null);
            }

            /** Represents a ToolchainConstraints. */
            class ToolchainConstraints implements IToolchainConstraints {

                /**
                 * Constructs a new ToolchainConstraints.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IToolchainConstraints);

                /** ToolchainConstraints allowedTools. */
                public allowedTools: string[];

                /** ToolchainConstraints allowedCommands. */
                public allowedCommands: string[];

                /**
                 * Creates a new ToolchainConstraints instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns ToolchainConstraints instance
                 */
                public static create(properties?: cordum.agent.v1.IToolchainConstraints): cordum.agent.v1.ToolchainConstraints;

                /**
                 * Encodes the specified ToolchainConstraints message. Does not implicitly {@link cordum.agent.v1.ToolchainConstraints.verify|verify} messages.
                 * @param message ToolchainConstraints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IToolchainConstraints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified ToolchainConstraints message, length delimited. Does not implicitly {@link cordum.agent.v1.ToolchainConstraints.verify|verify} messages.
                 * @param message ToolchainConstraints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IToolchainConstraints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a ToolchainConstraints message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns ToolchainConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.ToolchainConstraints;

                /**
                 * Decodes a ToolchainConstraints message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns ToolchainConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.ToolchainConstraints;

                /**
                 * Verifies a ToolchainConstraints message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a ToolchainConstraints message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns ToolchainConstraints
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.ToolchainConstraints;

                /**
                 * Creates a plain object from a ToolchainConstraints message. Also converts values to other types if specified.
                 * @param message ToolchainConstraints
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.ToolchainConstraints, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this ToolchainConstraints to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for ToolchainConstraints
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a DiffConstraints. */
            interface IDiffConstraints {

                /** DiffConstraints maxFiles */
                maxFiles?: (number|null);

                /** DiffConstraints maxLines */
                maxLines?: (number|null);

                /** DiffConstraints denyPathGlobs */
                denyPathGlobs?: (string[]|null);
            }

            /** Represents a DiffConstraints. */
            class DiffConstraints implements IDiffConstraints {

                /**
                 * Constructs a new DiffConstraints.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IDiffConstraints);

                /** DiffConstraints maxFiles. */
                public maxFiles: number;

                /** DiffConstraints maxLines. */
                public maxLines: number;

                /** DiffConstraints denyPathGlobs. */
                public denyPathGlobs: string[];

                /**
                 * Creates a new DiffConstraints instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns DiffConstraints instance
                 */
                public static create(properties?: cordum.agent.v1.IDiffConstraints): cordum.agent.v1.DiffConstraints;

                /**
                 * Encodes the specified DiffConstraints message. Does not implicitly {@link cordum.agent.v1.DiffConstraints.verify|verify} messages.
                 * @param message DiffConstraints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IDiffConstraints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified DiffConstraints message, length delimited. Does not implicitly {@link cordum.agent.v1.DiffConstraints.verify|verify} messages.
                 * @param message DiffConstraints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IDiffConstraints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a DiffConstraints message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns DiffConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.DiffConstraints;

                /**
                 * Decodes a DiffConstraints message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns DiffConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.DiffConstraints;

                /**
                 * Verifies a DiffConstraints message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a DiffConstraints message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns DiffConstraints
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.DiffConstraints;

                /**
                 * Creates a plain object from a DiffConstraints message. Also converts values to other types if specified.
                 * @param message DiffConstraints
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.DiffConstraints, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this DiffConstraints to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for DiffConstraints
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a PolicyConstraints. */
            interface IPolicyConstraints {

                /** PolicyConstraints budgets */
                budgets?: (cordum.agent.v1.IBudgetConstraints|null);

                /** PolicyConstraints sandbox */
                sandbox?: (cordum.agent.v1.ISandboxProfile|null);

                /** PolicyConstraints toolchain */
                toolchain?: (cordum.agent.v1.IToolchainConstraints|null);

                /** PolicyConstraints diff */
                diff?: (cordum.agent.v1.IDiffConstraints|null);

                /** PolicyConstraints redactionLevel */
                redactionLevel?: (string|null);
            }

            /** Represents a PolicyConstraints. */
            class PolicyConstraints implements IPolicyConstraints {

                /**
                 * Constructs a new PolicyConstraints.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IPolicyConstraints);

                /** PolicyConstraints budgets. */
                public budgets?: (cordum.agent.v1.IBudgetConstraints|null);

                /** PolicyConstraints sandbox. */
                public sandbox?: (cordum.agent.v1.ISandboxProfile|null);

                /** PolicyConstraints toolchain. */
                public toolchain?: (cordum.agent.v1.IToolchainConstraints|null);

                /** PolicyConstraints diff. */
                public diff?: (cordum.agent.v1.IDiffConstraints|null);

                /** PolicyConstraints redactionLevel. */
                public redactionLevel: string;

                /**
                 * Creates a new PolicyConstraints instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns PolicyConstraints instance
                 */
                public static create(properties?: cordum.agent.v1.IPolicyConstraints): cordum.agent.v1.PolicyConstraints;

                /**
                 * Encodes the specified PolicyConstraints message. Does not implicitly {@link cordum.agent.v1.PolicyConstraints.verify|verify} messages.
                 * @param message PolicyConstraints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IPolicyConstraints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified PolicyConstraints message, length delimited. Does not implicitly {@link cordum.agent.v1.PolicyConstraints.verify|verify} messages.
                 * @param message PolicyConstraints message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IPolicyConstraints, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a PolicyConstraints message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns PolicyConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.PolicyConstraints;

                /**
                 * Decodes a PolicyConstraints message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns PolicyConstraints
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.PolicyConstraints;

                /**
                 * Verifies a PolicyConstraints message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a PolicyConstraints message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns PolicyConstraints
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.PolicyConstraints;

                /**
                 * Creates a plain object from a PolicyConstraints message. Also converts values to other types if specified.
                 * @param message PolicyConstraints
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.PolicyConstraints, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this PolicyConstraints to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for PolicyConstraints
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a PolicyRemediation. */
            interface IPolicyRemediation {

                /** PolicyRemediation id */
                id?: (string|null);

                /** PolicyRemediation title */
                title?: (string|null);

                /** PolicyRemediation summary */
                summary?: (string|null);

                /** PolicyRemediation replacementTopic */
                replacementTopic?: (string|null);

                /** PolicyRemediation replacementCapability */
                replacementCapability?: (string|null);

                /** PolicyRemediation addLabels */
                addLabels?: ({ [k: string]: string }|null);

                /** PolicyRemediation removeLabels */
                removeLabels?: (string[]|null);
            }

            /** Represents a PolicyRemediation. */
            class PolicyRemediation implements IPolicyRemediation {

                /**
                 * Constructs a new PolicyRemediation.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IPolicyRemediation);

                /** PolicyRemediation id. */
                public id: string;

                /** PolicyRemediation title. */
                public title: string;

                /** PolicyRemediation summary. */
                public summary: string;

                /** PolicyRemediation replacementTopic. */
                public replacementTopic: string;

                /** PolicyRemediation replacementCapability. */
                public replacementCapability: string;

                /** PolicyRemediation addLabels. */
                public addLabels: { [k: string]: string };

                /** PolicyRemediation removeLabels. */
                public removeLabels: string[];

                /**
                 * Creates a new PolicyRemediation instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns PolicyRemediation instance
                 */
                public static create(properties?: cordum.agent.v1.IPolicyRemediation): cordum.agent.v1.PolicyRemediation;

                /**
                 * Encodes the specified PolicyRemediation message. Does not implicitly {@link cordum.agent.v1.PolicyRemediation.verify|verify} messages.
                 * @param message PolicyRemediation message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IPolicyRemediation, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified PolicyRemediation message, length delimited. Does not implicitly {@link cordum.agent.v1.PolicyRemediation.verify|verify} messages.
                 * @param message PolicyRemediation message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IPolicyRemediation, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a PolicyRemediation message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns PolicyRemediation
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.PolicyRemediation;

                /**
                 * Decodes a PolicyRemediation message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns PolicyRemediation
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.PolicyRemediation;

                /**
                 * Verifies a PolicyRemediation message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a PolicyRemediation message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns PolicyRemediation
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.PolicyRemediation;

                /**
                 * Creates a plain object from a PolicyRemediation message. Also converts values to other types if specified.
                 * @param message PolicyRemediation
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.PolicyRemediation, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this PolicyRemediation to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for PolicyRemediation
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a PolicyCheckResponse. */
            interface IPolicyCheckResponse {

                /** PolicyCheckResponse decision */
                decision?: (cordum.agent.v1.DecisionType|null);

                /** PolicyCheckResponse reason */
                reason?: (string|null);

                /** PolicyCheckResponse redactedContextPtr */
                redactedContextPtr?: (string|null);

                /** PolicyCheckResponse policySnapshot */
                policySnapshot?: (string|null);

                /** PolicyCheckResponse ruleId */
                ruleId?: (string|null);

                /** PolicyCheckResponse constraints */
                constraints?: (cordum.agent.v1.IPolicyConstraints|null);

                /** PolicyCheckResponse approvalRequired */
                approvalRequired?: (boolean|null);

                /** PolicyCheckResponse approvalRef */
                approvalRef?: (string|null);

                /** PolicyCheckResponse remediations */
                remediations?: (cordum.agent.v1.IPolicyRemediation[]|null);
            }

            /** Represents a PolicyCheckResponse. */
            class PolicyCheckResponse implements IPolicyCheckResponse {

                /**
                 * Constructs a new PolicyCheckResponse.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IPolicyCheckResponse);

                /** PolicyCheckResponse decision. */
                public decision: cordum.agent.v1.DecisionType;

                /** PolicyCheckResponse reason. */
                public reason: string;

                /** PolicyCheckResponse redactedContextPtr. */
                public redactedContextPtr: string;

                /** PolicyCheckResponse policySnapshot. */
                public policySnapshot: string;

                /** PolicyCheckResponse ruleId. */
                public ruleId: string;

                /** PolicyCheckResponse constraints. */
                public constraints?: (cordum.agent.v1.IPolicyConstraints|null);

                /** PolicyCheckResponse approvalRequired. */
                public approvalRequired: boolean;

                /** PolicyCheckResponse approvalRef. */
                public approvalRef: string;

                /** PolicyCheckResponse remediations. */
                public remediations: cordum.agent.v1.IPolicyRemediation[];

                /**
                 * Creates a new PolicyCheckResponse instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns PolicyCheckResponse instance
                 */
                public static create(properties?: cordum.agent.v1.IPolicyCheckResponse): cordum.agent.v1.PolicyCheckResponse;

                /**
                 * Encodes the specified PolicyCheckResponse message. Does not implicitly {@link cordum.agent.v1.PolicyCheckResponse.verify|verify} messages.
                 * @param message PolicyCheckResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IPolicyCheckResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified PolicyCheckResponse message, length delimited. Does not implicitly {@link cordum.agent.v1.PolicyCheckResponse.verify|verify} messages.
                 * @param message PolicyCheckResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IPolicyCheckResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a PolicyCheckResponse message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns PolicyCheckResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.PolicyCheckResponse;

                /**
                 * Decodes a PolicyCheckResponse message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns PolicyCheckResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.PolicyCheckResponse;

                /**
                 * Verifies a PolicyCheckResponse message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a PolicyCheckResponse message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns PolicyCheckResponse
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.PolicyCheckResponse;

                /**
                 * Creates a plain object from a PolicyCheckResponse message. Also converts values to other types if specified.
                 * @param message PolicyCheckResponse
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.PolicyCheckResponse, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this PolicyCheckResponse to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for PolicyCheckResponse
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a ListSnapshotsRequest. */
            interface IListSnapshotsRequest {
            }

            /** Represents a ListSnapshotsRequest. */
            class ListSnapshotsRequest implements IListSnapshotsRequest {

                /**
                 * Constructs a new ListSnapshotsRequest.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IListSnapshotsRequest);

                /**
                 * Creates a new ListSnapshotsRequest instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns ListSnapshotsRequest instance
                 */
                public static create(properties?: cordum.agent.v1.IListSnapshotsRequest): cordum.agent.v1.ListSnapshotsRequest;

                /**
                 * Encodes the specified ListSnapshotsRequest message. Does not implicitly {@link cordum.agent.v1.ListSnapshotsRequest.verify|verify} messages.
                 * @param message ListSnapshotsRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IListSnapshotsRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified ListSnapshotsRequest message, length delimited. Does not implicitly {@link cordum.agent.v1.ListSnapshotsRequest.verify|verify} messages.
                 * @param message ListSnapshotsRequest message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IListSnapshotsRequest, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a ListSnapshotsRequest message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns ListSnapshotsRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.ListSnapshotsRequest;

                /**
                 * Decodes a ListSnapshotsRequest message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns ListSnapshotsRequest
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.ListSnapshotsRequest;

                /**
                 * Verifies a ListSnapshotsRequest message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a ListSnapshotsRequest message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns ListSnapshotsRequest
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.ListSnapshotsRequest;

                /**
                 * Creates a plain object from a ListSnapshotsRequest message. Also converts values to other types if specified.
                 * @param message ListSnapshotsRequest
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.ListSnapshotsRequest, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this ListSnapshotsRequest to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for ListSnapshotsRequest
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a ListSnapshotsResponse. */
            interface IListSnapshotsResponse {

                /** ListSnapshotsResponse snapshots */
                snapshots?: (string[]|null);
            }

            /** Represents a ListSnapshotsResponse. */
            class ListSnapshotsResponse implements IListSnapshotsResponse {

                /**
                 * Constructs a new ListSnapshotsResponse.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: cordum.agent.v1.IListSnapshotsResponse);

                /** ListSnapshotsResponse snapshots. */
                public snapshots: string[];

                /**
                 * Creates a new ListSnapshotsResponse instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns ListSnapshotsResponse instance
                 */
                public static create(properties?: cordum.agent.v1.IListSnapshotsResponse): cordum.agent.v1.ListSnapshotsResponse;

                /**
                 * Encodes the specified ListSnapshotsResponse message. Does not implicitly {@link cordum.agent.v1.ListSnapshotsResponse.verify|verify} messages.
                 * @param message ListSnapshotsResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: cordum.agent.v1.IListSnapshotsResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified ListSnapshotsResponse message, length delimited. Does not implicitly {@link cordum.agent.v1.ListSnapshotsResponse.verify|verify} messages.
                 * @param message ListSnapshotsResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: cordum.agent.v1.IListSnapshotsResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a ListSnapshotsResponse message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns ListSnapshotsResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): cordum.agent.v1.ListSnapshotsResponse;

                /**
                 * Decodes a ListSnapshotsResponse message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns ListSnapshotsResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): cordum.agent.v1.ListSnapshotsResponse;

                /**
                 * Verifies a ListSnapshotsResponse message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a ListSnapshotsResponse message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns ListSnapshotsResponse
                 */
                public static fromObject(object: { [k: string]: any }): cordum.agent.v1.ListSnapshotsResponse;

                /**
                 * Creates a plain object from a ListSnapshotsResponse message. Also converts values to other types if specified.
                 * @param message ListSnapshotsResponse
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: cordum.agent.v1.ListSnapshotsResponse, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this ListSnapshotsResponse to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for ListSnapshotsResponse
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Represents a SafetyKernel */
            class SafetyKernel extends $protobuf.rpc.Service {

                /**
                 * Constructs a new SafetyKernel service.
                 * @param rpcImpl RPC implementation
                 * @param [requestDelimited=false] Whether requests are length-delimited
                 * @param [responseDelimited=false] Whether responses are length-delimited
                 */
                constructor(rpcImpl: $protobuf.RPCImpl, requestDelimited?: boolean, responseDelimited?: boolean);

                /**
                 * Creates new SafetyKernel service using the specified rpc implementation.
                 * @param rpcImpl RPC implementation
                 * @param [requestDelimited=false] Whether requests are length-delimited
                 * @param [responseDelimited=false] Whether responses are length-delimited
                 * @returns RPC service. Useful where requests and/or responses are streamed.
                 */
                public static create(rpcImpl: $protobuf.RPCImpl, requestDelimited?: boolean, responseDelimited?: boolean): SafetyKernel;

                /**
                 * Calls Check.
                 * @param request PolicyCheckRequest message or plain object
                 * @param callback Node-style callback called with the error, if any, and PolicyCheckResponse
                 */
                public check(request: cordum.agent.v1.IPolicyCheckRequest, callback: cordum.agent.v1.SafetyKernel.CheckCallback): void;

                /**
                 * Calls Check.
                 * @param request PolicyCheckRequest message or plain object
                 * @returns Promise
                 */
                public check(request: cordum.agent.v1.IPolicyCheckRequest): Promise<cordum.agent.v1.PolicyCheckResponse>;

                /**
                 * Calls Evaluate.
                 * @param request PolicyCheckRequest message or plain object
                 * @param callback Node-style callback called with the error, if any, and PolicyCheckResponse
                 */
                public evaluate(request: cordum.agent.v1.IPolicyCheckRequest, callback: cordum.agent.v1.SafetyKernel.EvaluateCallback): void;

                /**
                 * Calls Evaluate.
                 * @param request PolicyCheckRequest message or plain object
                 * @returns Promise
                 */
                public evaluate(request: cordum.agent.v1.IPolicyCheckRequest): Promise<cordum.agent.v1.PolicyCheckResponse>;

                /**
                 * Calls Explain.
                 * @param request PolicyCheckRequest message or plain object
                 * @param callback Node-style callback called with the error, if any, and PolicyCheckResponse
                 */
                public explain(request: cordum.agent.v1.IPolicyCheckRequest, callback: cordum.agent.v1.SafetyKernel.ExplainCallback): void;

                /**
                 * Calls Explain.
                 * @param request PolicyCheckRequest message or plain object
                 * @returns Promise
                 */
                public explain(request: cordum.agent.v1.IPolicyCheckRequest): Promise<cordum.agent.v1.PolicyCheckResponse>;

                /**
                 * Calls Simulate.
                 * @param request PolicyCheckRequest message or plain object
                 * @param callback Node-style callback called with the error, if any, and PolicyCheckResponse
                 */
                public simulate(request: cordum.agent.v1.IPolicyCheckRequest, callback: cordum.agent.v1.SafetyKernel.SimulateCallback): void;

                /**
                 * Calls Simulate.
                 * @param request PolicyCheckRequest message or plain object
                 * @returns Promise
                 */
                public simulate(request: cordum.agent.v1.IPolicyCheckRequest): Promise<cordum.agent.v1.PolicyCheckResponse>;

                /**
                 * Calls ListSnapshots.
                 * @param request ListSnapshotsRequest message or plain object
                 * @param callback Node-style callback called with the error, if any, and ListSnapshotsResponse
                 */
                public listSnapshots(request: cordum.agent.v1.IListSnapshotsRequest, callback: cordum.agent.v1.SafetyKernel.ListSnapshotsCallback): void;

                /**
                 * Calls ListSnapshots.
                 * @param request ListSnapshotsRequest message or plain object
                 * @returns Promise
                 */
                public listSnapshots(request: cordum.agent.v1.IListSnapshotsRequest): Promise<cordum.agent.v1.ListSnapshotsResponse>;
            }

            namespace SafetyKernel {

                /**
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#check}.
                 * @param error Error, if any
                 * @param [response] PolicyCheckResponse
                 */
                type CheckCallback = (error: (Error|null), response?: cordum.agent.v1.PolicyCheckResponse) => void;

                /**
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#evaluate}.
                 * @param error Error, if any
                 * @param [response] PolicyCheckResponse
                 */
                type EvaluateCallback = (error: (Error|null), response?: cordum.agent.v1.PolicyCheckResponse) => void;

                /**
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#explain}.
                 * @param error Error, if any
                 * @param [response] PolicyCheckResponse
                 */
                type ExplainCallback = (error: (Error|null), response?: cordum.agent.v1.PolicyCheckResponse) => void;

                /**
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#simulate}.
                 * @param error Error, if any
                 * @param [response] PolicyCheckResponse
                 */
                type SimulateCallback = (error: (Error|null), response?: cordum.agent.v1.PolicyCheckResponse) => void;

                /**
                 * Callback as used by {@link cordum.agent.v1.SafetyKernel#listSnapshots}.
                 * @param error Error, if any
                 * @param [response] ListSnapshotsResponse
                 */
                type ListSnapshotsCallback = (error: (Error|null), response?: cordum.agent.v1.ListSnapshotsResponse) => void;
            }
        }
    }
}

/** Namespace google. */
export namespace google {

    /** Namespace protobuf. */
    namespace protobuf {

        /** Properties of a Timestamp. */
        interface ITimestamp {

            /** Timestamp seconds */
            seconds?: (number|Long|null);

            /** Timestamp nanos */
            nanos?: (number|null);
        }

        /** Represents a Timestamp. */
        class Timestamp implements ITimestamp {

            /**
             * Constructs a new Timestamp.
             * @param [properties] Properties to set
             */
            constructor(properties?: google.protobuf.ITimestamp);

            /** Timestamp seconds. */
            public seconds: (number|Long);

            /** Timestamp nanos. */
            public nanos: number;

            /**
             * Creates a new Timestamp instance using the specified properties.
             * @param [properties] Properties to set
             * @returns Timestamp instance
             */
            public static create(properties?: google.protobuf.ITimestamp): google.protobuf.Timestamp;

            /**
             * Encodes the specified Timestamp message. Does not implicitly {@link google.protobuf.Timestamp.verify|verify} messages.
             * @param message Timestamp message or plain object to encode
             * @param [writer] Writer to encode to
             * @returns Writer
             */
            public static encode(message: google.protobuf.ITimestamp, writer?: $protobuf.Writer): $protobuf.Writer;

            /**
             * Encodes the specified Timestamp message, length delimited. Does not implicitly {@link google.protobuf.Timestamp.verify|verify} messages.
             * @param message Timestamp message or plain object to encode
             * @param [writer] Writer to encode to
             * @returns Writer
             */
            public static encodeDelimited(message: google.protobuf.ITimestamp, writer?: $protobuf.Writer): $protobuf.Writer;

            /**
             * Decodes a Timestamp message from the specified reader or buffer.
             * @param reader Reader or buffer to decode from
             * @param [length] Message length if known beforehand
             * @returns Timestamp
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): google.protobuf.Timestamp;

            /**
             * Decodes a Timestamp message from the specified reader or buffer, length delimited.
             * @param reader Reader or buffer to decode from
             * @returns Timestamp
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): google.protobuf.Timestamp;

            /**
             * Verifies a Timestamp message.
             * @param message Plain object to verify
             * @returns `null` if valid, otherwise the reason why it is not
             */
            public static verify(message: { [k: string]: any }): (string|null);

            /**
             * Creates a Timestamp message from a plain object. Also converts values to their respective internal types.
             * @param object Plain object
             * @returns Timestamp
             */
            public static fromObject(object: { [k: string]: any }): google.protobuf.Timestamp;

            /**
             * Creates a plain object from a Timestamp message. Also converts values to other types if specified.
             * @param message Timestamp
             * @param [options] Conversion options
             * @returns Plain object
             */
            public static toObject(message: google.protobuf.Timestamp, options?: $protobuf.IConversionOptions): { [k: string]: any };

            /**
             * Converts this Timestamp to JSON.
             * @returns JSON object
             */
            public toJSON(): { [k: string]: any };

            /**
             * Gets the default type url for Timestamp
             * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
             * @returns The default type url
             */
            public static getTypeUrl(typeUrlPrefix?: string): string;
        }

        /** Properties of a Struct. */
        interface IStruct {

            /** Struct fields */
            fields?: ({ [k: string]: google.protobuf.IValue }|null);
        }

        /** Represents a Struct. */
        class Struct implements IStruct {

            /**
             * Constructs a new Struct.
             * @param [properties] Properties to set
             */
            constructor(properties?: google.protobuf.IStruct);

            /** Struct fields. */
            public fields: { [k: string]: google.protobuf.IValue };

            /**
             * Creates a new Struct instance using the specified properties.
             * @param [properties] Properties to set
             * @returns Struct instance
             */
            public static create(properties?: google.protobuf.IStruct): google.protobuf.Struct;

            /**
             * Encodes the specified Struct message. Does not implicitly {@link google.protobuf.Struct.verify|verify} messages.
             * @param message Struct message or plain object to encode
             * @param [writer] Writer to encode to
             * @returns Writer
             */
            public static encode(message: google.protobuf.IStruct, writer?: $protobuf.Writer): $protobuf.Writer;

            /**
             * Encodes the specified Struct message, length delimited. Does not implicitly {@link google.protobuf.Struct.verify|verify} messages.
             * @param message Struct message or plain object to encode
             * @param [writer] Writer to encode to
             * @returns Writer
             */
            public static encodeDelimited(message: google.protobuf.IStruct, writer?: $protobuf.Writer): $protobuf.Writer;

            /**
             * Decodes a Struct message from the specified reader or buffer.
             * @param reader Reader or buffer to decode from
             * @param [length] Message length if known beforehand
             * @returns Struct
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): google.protobuf.Struct;

            /**
             * Decodes a Struct message from the specified reader or buffer, length delimited.
             * @param reader Reader or buffer to decode from
             * @returns Struct
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): google.protobuf.Struct;

            /**
             * Verifies a Struct message.
             * @param message Plain object to verify
             * @returns `null` if valid, otherwise the reason why it is not
             */
            public static verify(message: { [k: string]: any }): (string|null);

            /**
             * Creates a Struct message from a plain object. Also converts values to their respective internal types.
             * @param object Plain object
             * @returns Struct
             */
            public static fromObject(object: { [k: string]: any }): google.protobuf.Struct;

            /**
             * Creates a plain object from a Struct message. Also converts values to other types if specified.
             * @param message Struct
             * @param [options] Conversion options
             * @returns Plain object
             */
            public static toObject(message: google.protobuf.Struct, options?: $protobuf.IConversionOptions): { [k: string]: any };

            /**
             * Converts this Struct to JSON.
             * @returns JSON object
             */
            public toJSON(): { [k: string]: any };

            /**
             * Gets the default type url for Struct
             * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
             * @returns The default type url
             */
            public static getTypeUrl(typeUrlPrefix?: string): string;
        }

        /** Properties of a Value. */
        interface IValue {

            /** Value nullValue */
            nullValue?: (google.protobuf.NullValue|null);

            /** Value numberValue */
            numberValue?: (number|null);

            /** Value stringValue */
            stringValue?: (string|null);

            /** Value boolValue */
            boolValue?: (boolean|null);

            /** Value structValue */
            structValue?: (google.protobuf.IStruct|null);

            /** Value listValue */
            listValue?: (google.protobuf.IListValue|null);
        }

        /** Represents a Value. */
        class Value implements IValue {

            /**
             * Constructs a new Value.
             * @param [properties] Properties to set
             */
            constructor(properties?: google.protobuf.IValue);

            /** Value nullValue. */
            public nullValue?: (google.protobuf.NullValue|null);

            /** Value numberValue. */
            public numberValue?: (number|null);

            /** Value stringValue. */
            public stringValue?: (string|null);

            /** Value boolValue. */
            public boolValue?: (boolean|null);

            /** Value structValue. */
            public structValue?: (google.protobuf.IStruct|null);

            /** Value listValue. */
            public listValue?: (google.protobuf.IListValue|null);

            /** Value kind. */
            public kind?: ("nullValue"|"numberValue"|"stringValue"|"boolValue"|"structValue"|"listValue");

            /**
             * Creates a new Value instance using the specified properties.
             * @param [properties] Properties to set
             * @returns Value instance
             */
            public static create(properties?: google.protobuf.IValue): google.protobuf.Value;

            /**
             * Encodes the specified Value message. Does not implicitly {@link google.protobuf.Value.verify|verify} messages.
             * @param message Value message or plain object to encode
             * @param [writer] Writer to encode to
             * @returns Writer
             */
            public static encode(message: google.protobuf.IValue, writer?: $protobuf.Writer): $protobuf.Writer;

            /**
             * Encodes the specified Value message, length delimited. Does not implicitly {@link google.protobuf.Value.verify|verify} messages.
             * @param message Value message or plain object to encode
             * @param [writer] Writer to encode to
             * @returns Writer
             */
            public static encodeDelimited(message: google.protobuf.IValue, writer?: $protobuf.Writer): $protobuf.Writer;

            /**
             * Decodes a Value message from the specified reader or buffer.
             * @param reader Reader or buffer to decode from
             * @param [length] Message length if known beforehand
             * @returns Value
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): google.protobuf.Value;

            /**
             * Decodes a Value message from the specified reader or buffer, length delimited.
             * @param reader Reader or buffer to decode from
             * @returns Value
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): google.protobuf.Value;

            /**
             * Verifies a Value message.
             * @param message Plain object to verify
             * @returns `null` if valid, otherwise the reason why it is not
             */
            public static verify(message: { [k: string]: any }): (string|null);

            /**
             * Creates a Value message from a plain object. Also converts values to their respective internal types.
             * @param object Plain object
             * @returns Value
             */
            public static fromObject(object: { [k: string]: any }): google.protobuf.Value;

            /**
             * Creates a plain object from a Value message. Also converts values to other types if specified.
             * @param message Value
             * @param [options] Conversion options
             * @returns Plain object
             */
            public static toObject(message: google.protobuf.Value, options?: $protobuf.IConversionOptions): { [k: string]: any };

            /**
             * Converts this Value to JSON.
             * @returns JSON object
             */
            public toJSON(): { [k: string]: any };

            /**
             * Gets the default type url for Value
             * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
             * @returns The default type url
             */
            public static getTypeUrl(typeUrlPrefix?: string): string;
        }

        /** NullValue enum. */
        enum NullValue {
            NULL_VALUE = 0
        }

        /** Properties of a ListValue. */
        interface IListValue {

            /** ListValue values */
            values?: (google.protobuf.IValue[]|null);
        }

        /** Represents a ListValue. */
        class ListValue implements IListValue {

            /**
             * Constructs a new ListValue.
             * @param [properties] Properties to set
             */
            constructor(properties?: google.protobuf.IListValue);

            /** ListValue values. */
            public values: google.protobuf.IValue[];

            /**
             * Creates a new ListValue instance using the specified properties.
             * @param [properties] Properties to set
             * @returns ListValue instance
             */
            public static create(properties?: google.protobuf.IListValue): google.protobuf.ListValue;

            /**
             * Encodes the specified ListValue message. Does not implicitly {@link google.protobuf.ListValue.verify|verify} messages.
             * @param message ListValue message or plain object to encode
             * @param [writer] Writer to encode to
             * @returns Writer
             */
            public static encode(message: google.protobuf.IListValue, writer?: $protobuf.Writer): $protobuf.Writer;

            /**
             * Encodes the specified ListValue message, length delimited. Does not implicitly {@link google.protobuf.ListValue.verify|verify} messages.
             * @param message ListValue message or plain object to encode
             * @param [writer] Writer to encode to
             * @returns Writer
             */
            public static encodeDelimited(message: google.protobuf.IListValue, writer?: $protobuf.Writer): $protobuf.Writer;

            /**
             * Decodes a ListValue message from the specified reader or buffer.
             * @param reader Reader or buffer to decode from
             * @param [length] Message length if known beforehand
             * @returns ListValue
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): google.protobuf.ListValue;

            /**
             * Decodes a ListValue message from the specified reader or buffer, length delimited.
             * @param reader Reader or buffer to decode from
             * @returns ListValue
             * @throws {Error} If the payload is not a reader or valid buffer
             * @throws {$protobuf.util.ProtocolError} If required fields are missing
             */
            public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): google.protobuf.ListValue;

            /**
             * Verifies a ListValue message.
             * @param message Plain object to verify
             * @returns `null` if valid, otherwise the reason why it is not
             */
            public static verify(message: { [k: string]: any }): (string|null);

            /**
             * Creates a ListValue message from a plain object. Also converts values to their respective internal types.
             * @param object Plain object
             * @returns ListValue
             */
            public static fromObject(object: { [k: string]: any }): google.protobuf.ListValue;

            /**
             * Creates a plain object from a ListValue message. Also converts values to other types if specified.
             * @param message ListValue
             * @param [options] Conversion options
             * @returns Plain object
             */
            public static toObject(message: google.protobuf.ListValue, options?: $protobuf.IConversionOptions): { [k: string]: any };

            /**
             * Converts this ListValue to JSON.
             * @returns JSON object
             */
            public toJSON(): { [k: string]: any };

            /**
             * Gets the default type url for ListValue
             * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
             * @returns The default type url
             */
            public static getTypeUrl(typeUrlPrefix?: string): string;
        }
    }
}
