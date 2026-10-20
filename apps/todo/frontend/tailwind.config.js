/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,jsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: [
          "Inter",
          "system-ui",
          "-apple-system",
          "Segoe UI",
          "Roboto",
          "sans-serif",
        ],
      },
      colors: {
        star: {
          void: "#030508",
          panel: "#0c1018",
          edge: "#1a2233",
          glow: "#3d5afe",
          mist: "#8b9cb3",
        },
      },
      boxShadow: {
        panel: "0 0 0 1px rgba(255,255,255,0.06), 0 24px 48px rgba(0,0,0,0.45)",
        glow: "0 0 40px rgba(61, 90, 254, 0.15)",
      },
    },
  },
  plugins: [],
};
