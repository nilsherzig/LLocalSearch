export function changeDarkMode(isDarkMode: boolean) {
    if (typeof window === 'undefined') return;
    if (isDarkMode === undefined) {
        isDarkMode =
            localStorage.theme === 'dark' ||
            (!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches);
    }
    if (isDarkMode) {
        document.documentElement.classList.add('dark');
        localStorage.theme = 'dark';
    } else {
        document.documentElement.classList.remove('dark');
        localStorage.theme = 'light';
    }
}
