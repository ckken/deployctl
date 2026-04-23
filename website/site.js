const copyButtons = document.querySelectorAll("[data-copy-target]");

copyButtons.forEach((button) => {
  button.addEventListener("click", async () => {
    const target = document.getElementById(button.dataset.copyTarget);
    if (!target) return;

    const text =
      "value" in target && typeof target.value === "string"
        ? target.value
        : target.textContent || "";

    try {
      await navigator.clipboard.writeText(text.trim());
      const original = button.textContent;
      button.textContent = "已复制";
      setTimeout(() => {
        button.textContent = original;
      }, 1200);
    } catch (error) {
      console.error(error);
      button.textContent = "复制失败";
    }
  });
});
