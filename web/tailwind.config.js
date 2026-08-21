/** @type {import('tailwindcss').Config} */
const token = (name) => `var(${name})`;

export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        brand: {
          primary: token("--brand-primary"),
          "primary-strong": token("--brand-primary-strong"),
          "accent-light": token("--brand-accent-light"),
          bg: token("--brand-bg"),
          surface: token("--brand-surface"),
          "surface-2": token("--brand-surface-2"),
          border: token("--brand-border"),
          text: token("--brand-text"),
          "on-primary": token("--brand-on-primary"),
          muted: token("--brand-muted"),
          success: token("--brand-success"),
          "success-soft": token("--brand-success-soft"),
          warning: token("--brand-warning"),
          "warning-soft": token("--brand-warning-soft"),
          danger: token("--brand-danger"),
          "danger-soft": token("--brand-danger-soft"),
          "primary-soft": token("--brand-primary-soft"),
        },
      },
      maxWidth: {
        app: "80rem",
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "sans-serif"],
      },
    },
  },
  plugins: [],
};
