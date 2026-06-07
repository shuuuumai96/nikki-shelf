import { onBeforeUnmount, onMounted, ref } from "vue";

export function useOnlineStatus() {
  const isOffline = ref(!navigator.onLine);

  function updateOnlineStatus() {
    isOffline.value = !navigator.onLine;
  }

  onMounted(() => {
    updateOnlineStatus();
    window.addEventListener("online", updateOnlineStatus);
    window.addEventListener("offline", updateOnlineStatus);
  });

  onBeforeUnmount(() => {
    window.removeEventListener("online", updateOnlineStatus);
    window.removeEventListener("offline", updateOnlineStatus);
  });

  return { isOffline };
}
