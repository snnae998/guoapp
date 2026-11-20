package dev.openhongguo

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

// 用假的 Engine 代替 Go 的 core 库，检查参数编码、结果解码、错误映射和取消。
class ClientTest {
    private class FakeEngine(val reply: (method: String, args: JSONArray) -> String) : Engine {
        val calls = mutableListOf<Pair<String, String>>()

        override fun start(method: String, args: String, done: (String) -> Unit): () -> Unit {
            calls += method to args
            done(reply(method, JSONArray(args)))
            return {}
        }
    }

    private fun ok(result: Any?) = JSONObject().put("result", result ?: JSONObject.NULL).toString()

    private fun series(id: String) = JSONObject().put("id", id).put("title", "剧 $id").put("episode_count", 80)

    @Test
    fun searchEncodesArgumentsAndDecodesPage() = runBlocking {
        val engine = FakeEngine { _, _ ->
            ok(JSONObject().put("items", JSONArray().put(series("1"))).put("cursor", "c2").put("has_more", true))
        }
        val page = OpenhongguoClient(engine).search("短剧", SearchOptions(pageSize = 20, filters = listOf("a")))
        assertEquals("search" to """["短剧",{"pageSize":20,"filters":["a"]}]""", engine.calls.single())
        assertEquals("剧 1", page.items.single().title)
        assertEquals(80, page.items.single().episodeCount)
        assertEquals("c2", page.cursor)
        assertTrue(page.hasMore)
    }

    @Test
    fun missingAndNullFieldsBecomeZeroValues() = runBlocking {
        val engine = FakeEngine { _, _ ->
            ok(JSONObject().put("id", "1").put("subtitles", JSONObject.NULL).put("novel", JSONObject.NULL))
        }
        val s = OpenhongguoClient(engine).getSeries("1")
        assertEquals("", s.title)
        assertEquals(emptyList<String>(), s.subtitles)
        assertNull(s.novel)
        assertEquals(SeriesStatus.UPDATING, s.status)
    }

    @Test
    fun searchAllFollowsCursorAndDropsDuplicates() = runBlocking {
        val engine = FakeEngine { _, args ->
            when (args.getJSONObject(1).optString("cursor")) {
                "" -> ok(JSONObject().put("items", JSONArray().put(series("1")).put(series("2"))).put("cursor", "p2").put("has_more", true))
                "p2" -> ok(JSONObject().put("items", JSONArray().put(series("2")).put(series("3"))).put("has_more", false))
                else -> error("unexpected cursor")
            }
        }
        val ids = OpenhongguoClient(engine).searchAll("短剧").toList().map { it.id }
        assertEquals(listOf("1", "2", "3"), ids)
        assertEquals(2, engine.calls.size)
    }

    @Test
    fun errorsBecomeOpenhongguoException() = runBlocking {
        val engine = FakeEngine { _, _ ->
            """{"error":{"code":"RATE_LIMITED","message":"slow down","retryable":true,"retryAfterMs":1500}}"""
        }
        try {
            OpenhongguoClient(engine).hotSearch()
            fail("no exception")
        } catch (e: OpenhongguoException) {
            assertEquals(ErrorCode.RATE_LIMITED, e.code)
            assertEquals("slow down", e.message)
            assertTrue(e.retryable)
            assertEquals(1500L, e.retryAfterMs)
        }
    }

    @Test
    fun unknownErrorCodeMapsToUnknown() {
        assertEquals(ErrorCode.UNKNOWN, OpenhongguoException.parse("""{"code":"NEW_THING"}""").code)
        assertEquals(ErrorCode.UNKNOWN, OpenhongguoException.parse("not json").code)
    }

    @Test
    fun resolvePlaybackKeepsPlaybackWhenKeyFails() = runBlocking {
        val engine = FakeEngine { _, _ ->
            ok(
                JSONObject()
                    .put("playback", JSONObject().put("protected", true).put("streams", JSONArray().put(JSONObject().put("definition", "720p"))))
                    .put("contentKeyError", JSONObject().put("code", "DRM_PROTECTED").put("message", "bad key")),
            )
        }
        val r = OpenhongguoClient(engine).resolvePlayback("9")
        assertTrue(r.playback.protected)
        assertEquals("720p", r.playback.streams.single().definition)
        assertNull(r.contentKey)
        assertEquals(ErrorCode.DRM_PROTECTED, r.contentKeyError?.code)
    }

    @Test
    fun cancellingTheCoroutineCancelsTheCall() = runBlocking {
        val cancelled = CompletableDeferred<Unit>()
        val engine = Engine { _, _, _ -> { cancelled.complete(Unit) } }
        // UNDISPATCHED 让调用立即开始，挂起在等待结果处。
        val job = async(start = CoroutineStart.UNDISPATCHED) { OpenhongguoClient(engine).rankGroups() }
        job.cancel()
        withTimeout(5_000) { cancelled.await() }
    }

    @Test
    fun enumsAndOptionsRoundTrip() {
        val d = Device(brand = "Google", model = "Pixel 9")
        val o = ClientOptions(device = d, retryAttempts = 1).toJson()
        assertEquals("Pixel 9", o.getJSONObject("device").getString("model"))
        assertTrue(!o.has("apiOrigin"))
        val ep = Episode.fromJson(JSONObject().put("content_type", 7))
        assertEquals(ContentType.SHORT_PLAY, ep.contentType)
        assertEquals(7, Episode.fromJson(ep.toJson()).contentType.value)
    }
}
