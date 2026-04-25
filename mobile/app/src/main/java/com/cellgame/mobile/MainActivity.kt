package com.cellgame.mobile

import android.annotation.SuppressLint
import android.content.pm.ActivityInfo
import android.graphics.Bitmap
import android.net.http.SslError
import android.os.Bundle
import android.os.SystemClock
import android.view.View
import android.webkit.CookieManager
import android.webkit.SslErrorHandler
import android.webkit.WebChromeClient
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.TextView
import android.widget.Toast
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import com.google.android.material.button.MaterialButton
import com.google.android.material.progressindicator.LinearProgressIndicator

class MainActivity : AppCompatActivity() {
    companion object {
        private const val APP_USER_AGENT_SUFFIX = " CellGameAndroidWebView"
        private const val EXIT_CONFIRM_WINDOW_MS = 1800L
    }

    private lateinit var webView: WebView
    private lateinit var loadingBar: LinearProgressIndicator
    private lateinit var errorPanel: View
    private lateinit var errorMessage: TextView
    private lateinit var retryButton: MaterialButton

    private var pageLoadFailed = false
    private var lastBackPressedAt = 0L

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enforceLandscapeOrientation()
        WindowCompat.setDecorFitsSystemWindows(window, false)
        setContentView(R.layout.activity_main)

        webView = findViewById(R.id.gameWebView)
        loadingBar = findViewById(R.id.loadingBar)
        errorPanel = findViewById(R.id.errorPanel)
        errorMessage = findViewById(R.id.errorMessage)
        retryButton = findViewById(R.id.retryButton)

        configureWebView()
        retryButton.setOnClickListener { loadGame() }

        onBackPressedDispatcher.addCallback(
            this,
            object : OnBackPressedCallback(true) {
                override fun handleOnBackPressed() {
                    if (webView.canGoBack()) {
                        webView.goBack()
                    } else {
                        maybeExitApp()
                    }
                }
            },
        )

        if (savedInstanceState != null) {
            webView.restoreState(savedInstanceState)
        } else {
            loadGame()
        }
    }

    override fun onResume() {
        super.onResume()
        enforceLandscapeOrientation()
        enterImmersiveMode()
        webView.onResume()
    }

    override fun onPause() {
        webView.onPause()
        super.onPause()
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (hasFocus) {
            enforceLandscapeOrientation()
            enterImmersiveMode()
        }
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        webView.saveState(outState)
    }

    override fun onDestroy() {
        webView.stopLoading()
        webView.webChromeClient = null
        webView.webViewClient = WebViewClient()
        webView.destroy()
        super.onDestroy()
    }

    private fun enterImmersiveMode() {
        val controller = WindowInsetsControllerCompat(window, window.decorView)
        controller.systemBarsBehavior =
            WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        controller.hide(WindowInsetsCompat.Type.systemBars())
    }

    private fun enforceLandscapeOrientation() {
        requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
    }

    private fun maybeExitApp() {
        val now = SystemClock.elapsedRealtime()
        if (now - lastBackPressedAt <= EXIT_CONFIRM_WINDOW_MS) {
            finish()
            return
        }

        lastBackPressedAt = now
        Toast.makeText(this, "한 번 더 누르면 종료됩니다.", Toast.LENGTH_SHORT).show()
    }

    @SuppressLint("SetJavaScriptEnabled")
    private fun configureWebView() {
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)
        webView.setBackgroundColor(getColor(R.color.game_background))
        webView.isVerticalScrollBarEnabled = false
        webView.isHorizontalScrollBarEnabled = false
        webView.overScrollMode = View.OVER_SCROLL_NEVER

        with(webView.settings) {
            javaScriptEnabled = true
            domStorageEnabled = true
            useWideViewPort = true
            loadWithOverviewMode = true
            mediaPlaybackRequiresUserGesture = false
            allowFileAccess = false
            allowContentAccess = false
            javaScriptCanOpenWindowsAutomatically = false
            mixedContentMode =
                if (BuildConfig.DEBUG) {
                    WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE
                } else {
                    WebSettings.MIXED_CONTENT_NEVER_ALLOW
                }
            cacheMode = WebSettings.LOAD_DEFAULT
            if (!userAgentString.contains(APP_USER_AGENT_SUFFIX)) {
                userAgentString += APP_USER_AGENT_SUFFIX
            }
        }

        val cookieManager = CookieManager.getInstance()
        cookieManager.setAcceptCookie(true)
        cookieManager.setAcceptThirdPartyCookies(webView, false)

        webView.webChromeClient = object : WebChromeClient() {
            override fun onProgressChanged(view: WebView?, newProgress: Int) {
                if (newProgress >= 100) {
                    loadingBar.hide()
                    return
                }
                if (errorPanel.visibility != View.VISIBLE) {
                    loadingBar.show()
                    loadingBar.setProgressCompat(newProgress.coerceAtLeast(8), true)
                }
            }
        }

        webView.webViewClient = object : WebViewClient() {
            override fun onPageStarted(view: WebView?, url: String?, favicon: Bitmap?) {
                pageLoadFailed = false
                showLoading()
            }

            override fun onPageFinished(view: WebView?, url: String?) {
                injectNativeAppShim()
                if (!pageLoadFailed) {
                    errorPanel.visibility = View.GONE
                    loadingBar.hide()
                }
            }

            override fun onReceivedError(
                view: WebView?,
                request: WebResourceRequest?,
                error: WebResourceError?,
            ) {
                if (request?.isForMainFrame == true) {
                    pageLoadFailed = true
                    val description =
                        error?.description?.toString()?.takeIf { it.isNotBlank() }
                            ?: getString(R.string.web_error_generic)
                    showError(description)
                }
            }

            override fun onReceivedHttpError(
                view: WebView?,
                request: WebResourceRequest?,
                errorResponse: WebResourceResponse?,
            ) {
                if (request?.isForMainFrame == true) {
                    pageLoadFailed = true
                    showError(
                        getString(
                            R.string.web_error_http,
                            errorResponse?.statusCode ?: 0,
                        ),
                    )
                }
            }

            override fun onReceivedSslError(
                view: WebView?,
                handler: SslErrorHandler,
                error: SslError?,
            ) {
                pageLoadFailed = true
                handler.cancel()
                showError(getString(R.string.web_error_ssl))
            }
        }
    }

    private fun injectNativeAppShim() {
        val script =
            """
            (() => {
              try {
                window.__CELLGAME_NATIVE_APP__ = true;
                const centerPointer = () => {
                  const canvas = document.getElementById("gameCanvas");
                  if (!canvas) {
                    return;
                  }
                  const rect = canvas.getBoundingClientRect();
                  canvas.dispatchEvent(new MouseEvent("mousemove", {
                    bubbles: true,
                    cancelable: true,
                    clientX: rect.left + (rect.width / 2),
                    clientY: rect.top + (rect.height / 2),
                    view: window,
                  }));
                };
                const markFullscreen = () => {
                  const fullscreenKeys = ["fullscreenElement", "webkitFullscreenElement", "msFullscreenElement"];
                  for (const key of fullscreenKeys) {
                    try {
                      Object.defineProperty(document, key, {
                        configurable: true,
                        get: () => document.documentElement,
                      });
                    } catch (_) {}
                  }
                  try {
                    if (screen.orientation) {
                      Object.defineProperty(screen.orientation, "type", {
                        configurable: true,
                        get: () => "landscape-primary",
                      });
                      Object.defineProperty(screen.orientation, "angle", {
                        configurable: true,
                        get: () => 90,
                      });
                      screen.orientation.lock = async () => {};
                    }
                  } catch (_) {}
                  const prompt = document.getElementById("fullscreenPrompt");
                  if (prompt) {
                    prompt.classList.add("hidden");
                  }
                  const rotatePrompt = document.getElementById("rotatePrompt");
                  if (rotatePrompt) {
                    rotatePrompt.classList.add("hidden");
                  }
                };
                if (!window.__cellGameNativePatched) {
                  window.__cellGameNativePatched = true;
                  window.addEventListener("resize", () => requestAnimationFrame(centerPointer), { passive: true });
                  window.addEventListener("orientationchange", () => setTimeout(centerPointer, 120), { passive: true });
                  ["pointerup", "pointercancel", "touchend", "touchcancel"].forEach((eventName) => {
                    window.addEventListener(eventName, () => requestAnimationFrame(centerPointer), { passive: true });
                  });
                  document.addEventListener("visibilitychange", () => {
                    if (!document.hidden) {
                      requestAnimationFrame(centerPointer);
                    }
                  });
                }
                markFullscreen();
                centerPointer();
                window.__cellGameMarkFullscreen = markFullscreen;
                window.__cellGameCenterPointer = centerPointer;
              } catch (_) {}
            })();
            """.trimIndent()
        webView.evaluateJavascript(script, null)
    }

    private fun loadGame() {
        val baseUrl = BuildConfig.GAME_BASE_URL.trim()
        if (!baseUrl.startsWith("http://") && !baseUrl.startsWith("https://")) {
            showError(getString(R.string.web_error_invalid_url, baseUrl))
            return
        }
        if (baseUrl.contains("your-domain.example.com")) {
            showError(getString(R.string.web_error_release_placeholder))
            return
        }
        showLoading()
        webView.loadUrl(baseUrl)
    }

    private fun showLoading() {
        errorPanel.visibility = View.GONE
        loadingBar.show()
        loadingBar.setProgressCompat(12, false)
    }

    private fun showError(message: String) {
        loadingBar.hide()
        errorMessage.text = message
        errorPanel.visibility = View.VISIBLE
    }
}
