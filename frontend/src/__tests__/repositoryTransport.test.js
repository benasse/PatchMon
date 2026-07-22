import { describe, expect, it } from "vitest";
import {
	getRepositoryTransport,
	isHttpsRepository,
} from "../utils/repositoryTransport";

describe("repository transport", () => {
	it("identifies HTTPS from the backward-compatible isSecure field", () => {
		expect(
			getRepositoryTransport({ url: "http://mirror.example", isSecure: true }),
		).toBe("HTTPS");
	});

	it("falls back to the URL for older API responses", () => {
		expect(isHttpsRepository({ url: "HTTPS://mirror.example" })).toBe(true);
	});

	it("labels HTTP without describing it as insecure", () => {
		expect(
			getRepositoryTransport({ url: "http://mirror.example", isSecure: false }),
		).toBe("HTTP");
	});

	it("labels other protocols as without HTTPS", () => {
		expect(
			getRepositoryTransport({
				url: "mirror://mirrors.example",
				isSecure: false,
			}),
		).toBe("Without HTTPS");
	});
});
