export const isHttpsRepository = (repository) =>
	typeof repository?.isSecure === "boolean"
		? repository.isSecure
		: /^https:\/\//i.test(repository?.url || "");

export const getRepositoryTransport = (repository) => {
	if (isHttpsRepository(repository)) return "HTTPS";
	if (/^http:\/\//i.test(repository?.url || "")) return "HTTP";
	return "Without HTTPS";
};
