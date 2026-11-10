import { ref, onMounted, onBeforeUnmount } from "vue";
export function useMediaQuery(query) {
  const media = typeof window === "undefined" ? null : window.matchMedia(query);
  const matches = ref(media?.matches || false);
  const update = () => {
    matches.value = media.matches;
  };
  onMounted(() => media?.addEventListener("change", update));
  onBeforeUnmount(() => media?.removeEventListener("change", update));
  return matches;
}
