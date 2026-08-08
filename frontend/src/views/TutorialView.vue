<script setup lang="ts">
import { useI18n } from '../i18n'
const { t } = useI18n()
</script>

<template>
  <main class="page guide-page">
    <section class="subhero compact-subhero">
      <div><p class="eyebrow">{{ t.guide }}</p><h1>{{ t.guideTitle }}</h1><p>{{ t.guideDescription }}</p></div>
    </section>
    <div class="guide-grid">
      <aside class="guide-index panel"><span class="eyebrow">{{ t.contents }}</span><a href="#docker">01 {{ t.guideDocker }}</a><a href="#other">02 {{ t.guideOther }}</a><a href="#verify">03 {{ t.guideVerify }}</a></aside>
      <article class="guide-content">
        <section class="guide-notice">{{ t.guideNotice }}</section>
        <section id="docker" class="panel guide-step"><span>01</span><div><h2>{{ t.guideDocker }}</h2><p>{{ t.guideDockerBody }} <code>/etc/docker/daemon.json</code>，{{ t.guideDockerBodyAfter }}</p><pre><code>sudo systemctl restart docker</code></pre></div></section>
        <section id="other" class="panel guide-step"><span>02</span><div><h2>{{ t.guideOther }}</h2><p>{{ t.guideOtherBody }}</p></div></section>
        <section id="verify" class="panel guide-step"><span>03</span><div><h2>{{ t.guideVerify }}</h2><pre><code>docker pull hello-world
curl -fsS http://localhost/health</code></pre></div></section>
      </article>
    </div>
  </main>
</template>
