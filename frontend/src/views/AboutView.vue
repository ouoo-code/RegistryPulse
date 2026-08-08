<script setup lang="ts">
import { useI18n } from '../i18n'
const { t } = useI18n()
</script>

<template>
  <main class="page about-page">
    <section class="subhero compact-subhero"><div><p class="eyebrow">{{ t.about }}</p><h1>{{ t.aboutTitle }}</h1><p>{{ t.aboutDescription }}</p></div></section>
    <section class="about-grid">
      <article class="panel about-intro"><span class="about-mark">RP</span><h2>{{ t.aboutIntroTitle }}</h2><p>{{ t.aboutIntroBody }}</p></article>
      <article class="panel"><p class="eyebrow">{{ t.license }}</p><h2>{{ t.licenseTitle }}</h2><p>{{ t.licenseBody }}</p></article>
      <article class="panel about-project-info"><p class="eyebrow">{{ t.projectInfo }}</p><h2>{{ t.softwareName }} {{ t.softwareVersion }}</h2><dl><dt>{{ t.copyrightLabel }}</dt><dd>{{ t.copyrightHolder }}</dd><dt>{{ t.websiteLabel }}</dt><dd><a href="https://soft.54bb.com" target="_blank" rel="noopener noreferrer">{{ t.projectWebsite }}</a></dd><dt>{{ t.rightsLabel }}</dt><dd>{{ t.rightsReserved }}</dd></dl></article>
    </section>
  </main>
</template>
