/**
 * This script is not meant for general use and should only be used by the Encore core team
 * when releasing a new version of Encore. The assets which are downloaded using this script
 * are NOT covered by the same LICENSE described here: https://github.com/encoredev/encore/blob/main/LICENSE
 */

const https = require("https");
const fs = require("fs");
const path = require("path");

const download = (url, dest) => {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    https
      .get(url, (response) => {
        if (response.statusCode > 200) {
          reject(`Could not download ${url}\nFailed with status code: ${response.statusCode}
          `);
        }
        response.pipe(file);
        file.on("finish", () => {
          file.close(resolve);
        });
      })
      .on("error", (err) => {
        fs.unlink(dest);
        reject(err.message);
      });
  });
};

const fonts = [
  "beausite-classic/BeausiteClassicWeb-Ultrablack.woff2",
  "beausite-classic/BeausiteClassicWeb-Ultrablack.woff",
  "suisse-intl/SuisseIntl-Ultralight-WebS.woff2",
  "suisse-intl/SuisseIntl-Ultralight-WebS.woff",
  "suisse-intl/SuisseIntl-Ultralight-WebS.ttf",
  "suisse-intl/SuisseIntl-Ultralight-WebS.svg",
  "suisse-intl/SuisseIntl-Ultralight-WebS.eot",
  "suisse-intl/SuisseIntl-UltralightItalic-WebS.woff2",
  "suisse-intl/SuisseIntl-UltralightItalic-WebS.woff",
  "suisse-intl/SuisseIntl-UltralightItalic-WebS.ttf",
  "suisse-intl/SuisseIntl-UltralightItalic-WebS.svg",
  "suisse-intl/SuisseIntl-UltralightItalic-WebS.eot",
  "suisse-intl/SuisseIntl-Thin-WebS.woff2",
  "suisse-intl/SuisseIntl-Thin-WebS.woff",
  "suisse-intl/SuisseIntl-Thin-WebS.ttf",
  "suisse-intl/SuisseIntl-Thin-WebS.svg",
  "suisse-intl/SuisseIntl-Thin-WebS.eot",
  "suisse-intl/SuisseIntl-ThinItalic-WebS.woff2",
  "suisse-intl/SuisseIntl-ThinItalic-WebS.woff",
  "suisse-intl/SuisseIntl-ThinItalic-WebS.ttf",
  "suisse-intl/SuisseIntl-ThinItalic-WebS.svg",
  "suisse-intl/SuisseIntl-ThinItalic-WebS.eot",
  "suisse-intl/SuisseIntl-Light-WebS.woff2",
  "suisse-intl/SuisseIntl-Light-WebS.woff",
  "suisse-intl/SuisseIntl-Light-WebS.ttf",
  "suisse-intl/SuisseIntl-Light-WebS.svg",
  "suisse-intl/SuisseIntl-Light-WebS.eot",
  "suisse-intl/SuisseIntl-LightItalic-WebS.woff2",
  "suisse-intl/SuisseIntl-LightItalic-WebS.woff",
  "suisse-intl/SuisseIntl-LightItalic-WebS.ttf",
  "suisse-intl/SuisseIntl-LightItalic-WebS.svg",
  "suisse-intl/SuisseIntl-LightItalic-WebS.eot",
  "suisse-intl/SuisseIntl-Regular-WebS.woff2",
  "suisse-intl/SuisseIntl-Regular-WebS.woff",
  "suisse-intl/SuisseIntl-Regular-WebS.ttf",
  "suisse-intl/SuisseIntl-Regular-WebS.svg",
  "suisse-intl/SuisseIntl-Regular-WebS.eot",
  "suisse-intl/SuisseIntl-RegularItalic-WebS.woff2",
  "suisse-intl/SuisseIntl-RegularItalic-WebS.woff",
  "suisse-intl/SuisseIntl-RegularItalic-WebS.ttf",
  "suisse-intl/SuisseIntl-RegularItalic-WebS.svg",
  "suisse-intl/SuisseIntl-RegularItalic-WebS.eot",
  "suisse-intl/SuisseIntl-Medium-WebS.woff2",
  "suisse-intl/SuisseIntl-Medium-WebS.woff",
  "suisse-intl/SuisseIntl-Medium-WebS.ttf",
  "suisse-intl/SuisseIntl-Medium-WebS.svg",
  "suisse-intl/SuisseIntl-Medium-WebS.eot",
  "suisse-intl/SuisseIntl-MediumItalic-WebS.woff2",
  "suisse-intl/SuisseIntl-MediumItalic-WebS.woff",
  "suisse-intl/SuisseIntl-MediumItalic-WebS.ttf",
  "suisse-intl/SuisseIntl-MediumItalic-WebS.svg",
  "suisse-intl/SuisseIntl-MediumItalic-WebS.eot",
  "suisse-intl/SuisseIntl-SemiBold-WebS.woff2",
  "suisse-intl/SuisseIntl-SemiBold-WebS.woff",
  "suisse-intl/SuisseIntl-SemiBold-WebS.ttf",
  "suisse-intl/SuisseIntl-SemiBold-WebS.svg",
  "suisse-intl/SuisseIntl-SemiBold-WebS.eot",
  "suisse-intl/SuisseIntl-SemiBoldItalic-WebS.woff2",
  "suisse-intl/SuisseIntl-SemiBoldItalic-WebS.woff",
  "suisse-intl/SuisseIntl-SemiBoldItalic-WebS.ttf",
  "suisse-intl/SuisseIntl-SemiBoldItalic-WebS.svg",
  "suisse-intl/SuisseIntl-SemiBoldItalic-WebS.eot",
  "suisse-intl/SuisseIntl-Bold-WebS.woff2",
  "suisse-intl/SuisseIntl-Bold-WebS.woff",
  "suisse-intl/SuisseIntl-Bold-WebS.ttf",
  "suisse-intl/SuisseIntl-Bold-WebS.svg",
  "suisse-intl/SuisseIntl-Bold-WebS.eot",
  "suisse-intl/SuisseIntl-BoldItalic-WebS.woff2",
  "suisse-intl/SuisseIntl-BoldItalic-WebS.woff",
  "suisse-intl/SuisseIntl-BoldItalic-WebS.ttf",
  "suisse-intl/SuisseIntl-BoldItalic-WebS.svg",
  "suisse-intl/SuisseIntl-BoldItalic-WebS.eot",
  "suisse-intl/SuisseIntl-Black-WebS.woff2",
  "suisse-intl/SuisseIntl-Black-WebS.woff",
  "suisse-intl/SuisseIntl-Black-WebS.ttf",
  "suisse-intl/SuisseIntl-Black-WebS.svg",
  "suisse-intl/SuisseIntl-Black-WebS.eot",
  "suisse-intl/SuisseIntl-BlackItalic-WebS.woff2",
  "suisse-intl/SuisseIntl-BlackItalic-WebS.woff",
  "suisse-intl/SuisseIntl-BlackItalic-WebS.ttf",
  "suisse-intl/SuisseIntl-BlackItalic-WebS.svg",
  "suisse-intl/SuisseIntl-BlackItalic-WebS.eot",
  "suisse-intl/SuisseIntlMono-Thin-WebS.woff2",
  "suisse-intl/SuisseIntlMono-Thin-WebS.woff",
  "suisse-intl/SuisseIntlMono-Thin-WebS.ttf",
  "suisse-intl/SuisseIntlMono-Thin-WebS.svg",
  "suisse-intl/SuisseIntlMono-Thin-WebS.eot",
  "suisse-intl/SuisseIntlMono-Regular-WebS.woff2",
  "suisse-intl/SuisseIntlMono-Regular-WebS.woff",
  "suisse-intl/SuisseIntlMono-Regular-WebS.ttf",
  "suisse-intl/SuisseIntlMono-Regular-WebS.svg",
  "suisse-intl/SuisseIntlMono-Regular-WebS.eot",
  "suisse-intl/SuisseIntlMono-Bold-WebS.woff2",
  "suisse-intl/SuisseIntlMono-Bold-WebS.woff",
  "suisse-intl/SuisseIntlMono-Bold-WebS.ttf",
  "suisse-intl/SuisseIntlMono-Bold-WebS.svg",
  "suisse-intl/SuisseIntlMono-Bold-WebS.eot",
];

Promise.all(
  fonts.map((font) => {
    const [dir, fileName] = font.split("/");
    return download(
      `https://encore.dev/assets/fonts/${font}`,
      path.join(__dirname, "public", "fonts", dir, fileName)
    );
  })
)
  .then(() => console.log("\x1b[32m", "Fonts downloaded successfully\n"))
  .catch((err) => {
    console.log(err);
    process.exit(1);
  });
