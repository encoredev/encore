const defaultTheme = require("tailwindcss/defaultTheme");
const plugin = require("tailwindcss/plugin");

var colors = {
  white: "#EEEEE1",
  black: "#111111",
  orange: "#FF8719",
  yellow: "#FFE87A",
  green: "#D4DB00",
  lightblue: "#93C0FF",
  blue: "#4651FF",

  codeorange: "#FFB84A",
  codeyellow: "#E9E23D",
  codegreen: "#B3D77E",
  codeblue: "#6D89FF",
  codepurple: "#A36C8C",

  lightgray: "#CDCDC2",

  red: "#F05C48",

  "validation-fail": "#CB1010",
  "validation-pass": "#167C0D",

  transparent: "transparent",
  current: "currentColor",
};

// These gradient have been doubled
// and then we need to apply a background scale of 50% to get the original gradient
// this is to allow for the animation
var gradientsStops = {
  // Original `${colors.orange} 0.12%, ${colors.blue} 16.69%, ${colors.lightblue} 34.29%, ${colors.green} 50.85%, ${colors.yellow} 66.9%, ${colors.orange} 82.43%, ${colors.blue} 99.51%`,
  "brandient-slice": `${colors.orange} 0%, ${colors.yellow} 18.862%, ${colors.green} 38.365%, ${colors.lightblue} 58.483%, ${colors.blue} 79.870%, ${colors.orange} 100%`,
  "brandient-full": `${colors.orange} 0%, ${colors.blue} 20.13%, ${colors.lightblue} 41.517%, ${colors.green} 61.635%, ${colors.yellow} 81.138%, ${colors.orange} 100%`,
};

// If true the user can toggle dark mode in the UI
// otherwise we pick it up from <html class="dark">
// FIXME(dom): For final release set this to false
const userTogglableDarkMode = true;

module.exports = {
  // Disable tailwindcss's own inbuilt dark mode variant, so we can
  // add our own later which understands our `DarkSection` component.
  darkMode: "disabled",

  content: ["./src/**/*.{js,jsx,ts,tsx}"],

  theme: {
    colors: colors,
    fontFamily: {
      sans: ["Suisse Intl", ...defaultTheme.fontFamily.sans],
      header: ["Beausite Classic", ...defaultTheme.fontFamily.sans],
      mono: ["Suisse Intl Mono", ...defaultTheme.fontFamily.mono],
    },
    // These are our font sizes, use with a prefix of 'text-'

    // This the default spacing from tailwind, I've just removed all the ones we're not using for now.
    // This affects anything with like: 'padding', 'margin', 'width', 'height', 'maxHeight', 'gap', 'insert', 'space', 'translate'
    // See https://tailwindcss.com/docs/customizing-spacing#default-spacing-scale for the original lis
    spacing: {
      ...defaultTheme.spacing,
      12.5: "3.125rem",
      15: "3.75rem", // 60px (i.e. our input height)
      18: "4.5rem",
      25: "6.25rem",
      30: "7.5rem",
      50: "12.5rem",

      // Helpers for working with borders (which are defined in px not rem in default tailwind)
      "1px": "1px",
      "2px": "2px",
      "3px": "3px",
      "4px": "4px",
      "8px": "8px",
      "10px": "10px",
      "12px": "12px",
      "14px": "14px",
      "16px": "16px",
      "18px": "18px",
      "20px": "20px",
      "30px": "30px",
      "40px": "40px",

      "600px": "600px", // this is so we can animate the menu height

      // Utilities for using the grid gap/col
      "layout-grid-col":
        "calc((var(--layout-grid-col) / var(--layout-width)) * 100vw)",
      "layout-grid-gap":
        "calc((var(--layout-grid-gap) / var(--layout-width)) * 100vw)",
      "layout-grid-margin":
        "calc((var(--layout-grid-margin) / var(--layout-width)) * 100vw)",

      // Add gc-1 through gc-12 as spacing units ("grid cols"), that represent the width of
      // N grid columns in the layout grid.
      ...(() => {
        const entries = {};
        for (let i = 1; i <= 12; i++) {
          entries[
            `gc-${i}`
          ] = `calc((var(--layout-grid-col)*${i} + var(--layout-grid-gap)*${
            i - 1
          }) / var(--layout-width) * 100vw)`;
          entries[
            `gh-${i}`
          ] = `calc((var(--layout-grid-col)*${i} + var(--layout-grid-gap)*${
            i - 1
          }) / var(--layout-width) * 100vw / 2)`;
          entries[
            `gi-${i}`
          ] = `calc((var(--layout-grid-col)*${i} + var(--layout-grid-gap)*${i}) / var(--layout-width) * 100vw)`;
        }
        return entries;
      })(),
    },

    boxShadow: {
      brandy: `8px 8px 0 ${colors.black}`,
    },

    extend: {
      fontSize: {
        "headline-xxl": ["80px", "90%"], // Main Headline
        "headline-xl": ["50px", "90%"], // XLarge headline
        "headline-l": ["38px", "90%"], // Large headline
        heading: ["28px", "90%"], // Medium heading
        "heading-s": ["18px", "90%"], // Small heading
        "lead-xl": ["50px", "120%"], // XLarge lead copy
        "lead-l": ["38px", "120%"], // Large lead copy
        lead: ["28px", "120%"], // Medium lead copy
        "lead-s": ["18px", "120%"], // Small lead copy
        "lead-xs": ["16px", "120%"], // Xsmall lead copy
        "lead-xxs": ["14px", "125%"], // Xxsmall lead copy
        "body-l": ["26px", "120%"], // Large body copy
        body: ["16px", "120%"], // Body copy
        blog: ["18px", "140%"], // Blog copy
        list: ["14px", "140%"], // List copy
        "form-label": ["12px", "233%"],
        "code-s": ["14px", "120%"],
        "code-xs": ["12px", "120%"],

        "mobile-headline": ["52px", "90%"], // Large headline
        "mobile-heading": ["40px", "90%"], // Medium heading
        "mobile-lead-l": ["28px", "120%"], // Large lead copy
        "mobile-lead": ["20px", "120%"], // Medium lead copy
        "mobile-lead-s": ["16px", "120%"], // Small lead copy
        "mobile-lead-xs": ["14px", "20px"], // Small lead copy
        "mobile-body-l": ["20px", "120%"], // Large Body copy
        "mobile-blog": ["16px", "140%"], // Blog copy
        "mobile-body": ["16px", "120%"], // Body Copy
        "mobile-list": ["12px", "120%"], // List copy
        "mobile-form-label": ["12px", "233%"],
        "mobile-code-s": ["12px", "120%"],
        "mobile-code-xms": ["8px", "120%"],
        "mobile-code-xs": ["6px", "120%"],
        "mobile-menu": ["28px", "100%"],
      },
      screens: {
        mobile: { max: "670px" },
        d: { min: "671px" },
      },
      width: {
        sidebar: "250px",
      },
      height: {
        "nav-bar": "var(--nav-bar-height)",
        "full-minus-nav": "calc(100vh - var(--nav-bar-height))",
      },
      minWidth: {
        sidebar: "250px",
        pageNav: "200px",
      },
      fontFamily: {
        sans: ["Suisse Intl", ...defaultTheme.fontFamily.sans],
        header: ["Beausite Classic", ...defaultTheme.fontFamily.sans],
        mono: ["Suisse Intl Mono", ...defaultTheme.fontFamily.mono],
      },
      letterSpacing: {
        tightish: "-0.03em",
        tightly: "-0.02em",
      },
      transitionProperty: {
        "max-height": "max-height",
      },
      typography: (theme) => ({
        DEFAULT: {
          css: {
            "--tw-prose-pre-bg": theme("colors.black"),
            "--tw-prose-body": theme("colors.black"),
            "--tw-prose-pre-code": theme("colors.white"),
            "--tw-prose-bullets": theme("colors.black"),

            a: { textDecoration: "none" },
            li: {},
          },
        },
      }),
    },
    backgroundImage: {
      ...defaultTheme.backgroundImage,
      "brandient-slice": `linear-gradient(to right, ${gradientsStops["brandient-full"]})`,
      "brandient-full": `linear-gradient(to right, ${gradientsStops["brandient-full"]})`,
    },

    columns: defaultTheme.columns,
    blur: defaultTheme.blur,
    borderRadius: defaultTheme.borderRadius,
    borderWidth: {
      ...defaultTheme.borderWidth,
      3: "3px",
      5: "5px",
      6: "6px",

      // Ensure the default border does not go below 1px as it starts looking too thin.
      DEFAULT: `max(1px, ${defaultTheme.borderWidth.DEFAULT})`,
    },
    lineHeight: defaultTheme.lineHeight,
    outlineOffset: defaultTheme.outlineOffset,
    outlineWidth: defaultTheme.outlineWidth,
    ringWidth: defaultTheme.ringWidth,
    ringOffsetWidth: defaultTheme.ringOffsetWidth,
    tooltipArrows: (theme) => ({
      "gray-400-arrow": {
        borderColor: theme("colors.gray.400"),
        borderWidth: 1,
        backgroundColor: theme("colors.white"),
        size: 10,
        offset: 2,
      },
    }),
  },

  variants: {
    visibility: ["responsive", "group-hover"],
    extend: {
      border: ["last", "first"],
      typography: ["dark"],
    },
  },
  plugins: [
    require("@tailwindcss/forms"),
    require("@tailwindcss/typography"),
    require("tailwind-scrollbar"),
    plugin(function ({ addUtilities, matchUtilities, addVariant, theme }) {
      addUtilities({
        ".noisify": { zIndex: -200 },
        ".list-brandient": {},
        ".text-brandient": {
          color: "transparent",
          backgroundClip: "text",
          backgroundImage:
            "linear-gradient(to right, var(--tw-gradient-stops))",
        },
        ".brandient-1": {
          "--tw-gradient-stops": gradientsStops["brandient-full"],
        },
        ".brandient-2": {
          "--tw-gradient-stops": gradientsStops["brandient-full"],
        },
        ".brandient-3": {
          "--tw-gradient-stops": gradientsStops["brandient-full"],
        },
        ".brandient-4": {
          "--tw-gradient-stops": gradientsStops["brandient-full"],
        },
        ".brandient-5": {
          "--tw-gradient-stops": gradientsStops["brandient-full"],
        },
        ".brandient-full": {
          "--tw-gradient-stops": gradientsStops["brandient-full"],
        },

        // These are defined in global.css, but are added here
        // in order to get auto-complete working
        ".underline-bar": {},
        ".link-brandient": {},
      });

      // Add our own dark mode variant which
      // understands when it's inside a dark section

      addVariant("dark", [
        userTogglableDarkMode
          ? ".dark &"
          : "@media (prefers-color-scheme: dark)",
        ".dark-section &",
      ]);

      matchUtilities(
        {
          "underline-bar-height": (value) => {
            return {
              "--underline-bar-height": value,
            };
          },
        },
        { values: theme("borderWidth") }
      );

      matchUtilities(
        {
          elevate: (value) => {
            return {
              "--elevate-height": value,
            };
          },
        },
        { values: theme("borderWidth") }
      );
    }),
  ],
};
