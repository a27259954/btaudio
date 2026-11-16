// Fails when a language is missing a key that English has (or has extras).
import {dicts, LANGUAGES} from "../src/i18n.js";

const source = Object.keys(dicts.en);
let ok = true;
for (const [code] of LANGUAGES) {
  const d = dicts[code];
  if (!d) {
    console.error(`missing dictionary: ${code}`);
    ok = false;
    continue;
  }
  const missing = source.filter((k) => !(k in d));
  const extra = Object.keys(d).filter((k) => !(k in dicts.en));
  if (missing.length) console.error(`${code}: missing ${missing.join(", ")}`);
  if (extra.length) console.error(`${code}: unknown ${extra.join(", ")}`);
  ok &&= !missing.length && !extra.length;
}
if (!ok) process.exit(1);
console.log(`i18n ok: ${LANGUAGES.length} languages × ${source.length} keys`);
