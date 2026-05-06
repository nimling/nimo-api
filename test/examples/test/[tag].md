---
aside: false
outline: false
title: Example API
---

<script setup lang="ts">
import { useRoute, useData } from 'vitepress';
import spec from './spec.json';

const route = useRoute();
const { isDark } = useData();

const tag = route.data.params.tag
</script>

<OASpec :spec="spec" :tags="[tag]" :isDark="isDark" hide-info hide-servers hide-paths-summary />