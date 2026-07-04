// Ported from github.com/veil-proto/veil/link/link_test.go — same cases,
// same sample config, so the TS codec is verified against the same fixture
// the Go implementation is.
import { describe, expect, it } from "vitest";
import { decode, encode, SCHEME } from "./veil-link";

const sampleConf = `[Interface]
PrivateKey = 40e859dacd48da2172d6e0e8744c9e33307634759f86442754dd17e404d92e5f
Address = 10.8.0.2/24
NID = aa11
NetSecret = bb22

[Peer]
PublicKey = 990e2b3f56b5625d9f177b46f74ed8d25e94519abcb081877b54009d87e0517e
Endpoint = vpn.example.com:51820
AllowedIPs = 0.0.0.0/0
`;

describe("veil-link", () => {
  it("round-trips config text and name", () => {
    const link = encode(sampleConf, "Home Server");
    expect(link.startsWith(SCHEME)).toBe(true);
    const { configText, name } = decode(link);
    expect(configText).toBe(sampleConf);
    expect(name).toBe("Home Server");
  });

  it("round-trips with no name", () => {
    const { configText, name } = decode(encode(sampleConf, ""));
    expect(configText).toBe(sampleConf);
    expect(name).toBe("");
  });

  it("tolerates surrounding whitespace", () => {
    const link = `  ${encode(sampleConf, "x")}\n`;
    expect(() => decode(link)).not.toThrow();
  });

  it.each([
    "",
    "https://example.com",
    "veil://", // empty body
    "veil://!!!not-base64", // bad encoding
  ])("rejects %s", (input) => {
    expect(() => decode(input)).toThrow();
  });
});
