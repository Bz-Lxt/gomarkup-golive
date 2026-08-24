<template>
  <main class="flex min-h-screen w-full items-center justify-center p-6">
    <section class="w-full max-w-lg rounded-2xl border border-danger/40 bg-panel p-8 shadow-glow">
      <p class="font-mono text-xs uppercase tracking-[0.3em] text-danger">capability miss</p>
      <h1 class="mt-3 font-display text-3xl text-ink">当前浏览器没有 WebTransport</h1>
      <p class="mt-4 text-sm leading-7 text-mute">
        本控制台依赖 HTTP/3 上的 WebTransport。Chrome 97+、Edge 97+、Firefox 114+、Safari 26.4+ 可用。
        Safari ≤26.3 与多数内置 WebView 会走到这一页。请换用支持的浏览器，而不是继续点击已禁用的连接按钮。
      </p>
    </section>
  </main>
</template>
