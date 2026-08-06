/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        primary: {
          DEFAULT: "#1a7f5a",
          hover: "#166b4c",
          light: "#e3f2ea",
        },
        warning: "#b45309",
        danger: "#b91c1c",
        surface: "#faf9f7",
      },
    },
  },
  plugins: [],
};
