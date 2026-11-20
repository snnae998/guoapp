package dev.openhongguo

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.io.File
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import org.junit.runner.RunWith

// 在设备上走通 Go 的 core 库：回放 bindings/mobile/fixtureserver 的样本。
// 先在电脑上运行 go run ./bindings/mobile/fixtureserver -addr 127.0.0.1:8790，
// 模拟器通过 10.0.2.2 访问它；其他地址用 -e fixtureOrigin 传入。
@RunWith(AndroidJUnit4::class)
class FixtureTest {
    private val origin = InstrumentationRegistry.getArguments().getString("fixtureOrigin") ?: "http://10.0.2.2:8790"
    private val context = InstrumentationRegistry.getInstrumentation().targetContext

    private fun client(api: String = origin, device: Device? = null): Pair<OpenhongguoClient, File> {
        val profile = File(context.cacheDir, "fixture-${System.nanoTime()}/anonymous.json")
        val options = ClientOptions(apiOrigin = api, registerOrigin = api, distributionOrigin = origin, retryAttempts = 1, device = device)
        return OpenhongguoClient.create(profile, options) to profile
    }

    @Test
    fun searchSeriesAndEpisodes() = runBlocking {
        val (c, profile) = client()
        val page = c.search("短剧", SearchOptions(pageSize = 20))
        assertEquals("7100000000000101", page.items.first().id)
        assertEquals("Fixture Lantern Court", page.items.first().title)
        assertTrue("注册得到的设备存进了档案", profile.exists())

        val episodes = c.listEpisodes("7100000000000101")
        assertEquals(1, episodes.items.first().number)
    }

    @Test
    fun protectedPlaybackWithoutKey() = runBlocking {
        val r = client().first.resolvePlayback("7100000000000201")
        assertTrue(r.playback.protected)
        assertNull(r.contentKey) // 样本里的密钥材料是假的
        assertEquals(ErrorCode.DRM_PROTECTED, r.contentKeyError?.code)
    }

    @Test
    fun rateLimited() = runBlocking {
        try {
            client(api = "$origin/limited").first.search("短剧")
            fail("no exception")
        } catch (e: OpenhongguoException) {
            assertEquals(ErrorCode.RATE_LIMITED, e.code)
            assertTrue(e.retryable)
        }
    }

    @Test
    fun invalidDevice() {
        try {
            client(device = Device(model = "x; evil"))
            fail("no exception")
        } catch (e: OpenhongguoException) {
            assertEquals(ErrorCode.INVALID_OPTIONS, e.code)
        }
    }
}
