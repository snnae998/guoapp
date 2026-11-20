package dev.openhongguo

import org.json.JSONObject

/** 错误码，对应 Go SDK 的哨兵错误。 */
enum class ErrorCode {
    INVALID_OPTIONS,
    PROFILE_NOT_FOUND,
    SIGNER_UNAVAILABLE,
    AUTHENTICATION_REQUIRED,
    NOT_PROTECTED,
    DRM_PROTECTED,
    ANONYMOUS_LIMITED,
    RATE_LIMITED,
    TRANSPORT,
    SIGNATURE_REJECTED,
    CONTENT_UNAVAILABLE,
    PARAM_INVALID,
    CHALLENGE_REQUIRED,
    DEVICE_REJECTED,
    END_OF_LIST,
    ALBUM_UNAVAILABLE,

    /** 调用被取消。协程取消时抛出的是 CancellationException，不会见到这个错误码。 */
    CANCELLED,
    UNKNOWN,
    ;

    internal companion object {
        fun of(name: String): ErrorCode = entries.firstOrNull { it.name == name } ?: UNKNOWN
    }
}

/** OpenhongguoException 是 SDK 返回的错误。 */
class OpenhongguoException(
    /** 错误码，可以据此判断原因，而不必解析 message。 */
    val code: ErrorCode,
    message: String,
    /** 原样重试是否可能成功（限流、网络错误、服务端 5xx）。 */
    val retryable: Boolean = false,
    /** 服务端建议的等待时间，单位毫秒；0 表示没有建议。 */
    val retryAfterMs: Long = 0,
) : Exception(message) {
    internal companion object {
        fun fromJson(o: JSONObject) = OpenhongguoException(
            code = ErrorCode.of(o.optString("code")),
            message = o.optString("message"),
            retryable = o.optBoolean("retryable"),
            retryAfterMs = o.optLong("retryAfterMs"),
        )

        /** 解析 Go 端错误信息里的 JSON；不是 JSON 时归为 UNKNOWN。 */
        fun parse(message: String?): OpenhongguoException =
            try {
                fromJson(JSONObject(message ?: ""))
            } catch (e: Exception) {
                OpenhongguoException(ErrorCode.UNKNOWN, message ?: "unknown error")
            }
    }
}
