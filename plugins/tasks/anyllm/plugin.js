// AnyLLM video generation. The channel base URL is https://anyllm.cc/api/v1.
// Account, token-balance, and material-library endpoints are not generation
// tasks, so this driver does not expose them.

const VIDEO_MODELS = {
  "doubao-seedance-2-0-260128": { resolutions: ["480p", "720p", "1080p", "4k"], minDuration: 4, maxDuration: 15 },
  "doubao-seedance-2-0-fast-260128": { resolutions: ["480p", "720p"], minDuration: 4, maxDuration: 15 },
  "doubao-seedance-2-0-mini-260615": { resolutions: ["480p", "720p"], minDuration: 4, maxDuration: 15 },
  "doubao-seedance-2-5-260628": { resolutions: ["480p", "720p", "1080p"], minDuration: 4, maxDuration: 30 },
};
const RESOLUTIONS = ["480p", "720p", "1080p", "4k"];
const RATIOS = ["16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"];

export const meta = {
  apiVersion: 1,
  key: "anyllm",
  name: "AnyLLM",
  icon: "Doubao.Color",
  description: {
    en: "AnyLLM Seedance video generation",
    zh: "AnyLLM Seedance 视频生成",
  },
  version: "1.0.0",
  author: { name: "AnyLLM" },
  baseUrl: "https://anyllm.cc/api/v1",
  models: Object.keys(VIDEO_MODELS),
  fetchMode: "per_task",
  upstreams: ["vendor", "new_api"],
  usageSchema: {
    resolution: {
      enum: RESOLUTIONS,
      enumLabels: {
        "480p": { en: "480p", zh: "480p" },
        "720p": { en: "720p", zh: "720p" },
        "1080p": { en: "1080p", zh: "1080p" },
        "4k": { en: "4k", zh: "4k" },
      },
      description: { en: "Output video resolution", zh: "输出视频分辨率" },
    },
    duration: {
      type: "number",
      unit: "second",
      description: { en: "Requested video duration in seconds; smart selection is billed at the model maximum", zh: "请求的视频时长（秒）；智能时长按模型最长时长计费" },
    },
    video_input: {
      enum: ["none", "video"],
      enumLabels: {
        none: { en: "No reference video", zh: "无参考视频" },
        video: { en: "With reference video", zh: "有参考视频" },
      },
      description: { en: "Whether the request includes a reference video", zh: "请求是否包含参考视频" },
    },
  },
  usageExamples: [
    { label: "2.0 5s 720p", facts: { resolution: "720p", duration: 5, video_input: "none" } },
    { label: "2.0 5s 1080p", facts: { resolution: "1080p", duration: 5, video_input: "none" } },
    { label: "2.0 5s 4k + video", facts: { resolution: "4k", duration: 5, video_input: "video" } },
    { label: "2.5 5s 720p", facts: { resolution: "720p", duration: 5, video_input: "none" } },
  ],
  routes: [
    { method: "POST", path: "/basamp/api/v3/contents/generations/tasks", type: "submit", decode: "createTask", render: "taskCreated" },
    { method: "GET", path: "/basamp/api/v3/contents/generations/tasks/:task_id", type: "query", render: "taskStatus" },
  ],
  protocols: [{ name: "openai_responses", supports: ["stream", "sync", "background"] }, "openai_video"],
};

function trimmed(value) {
  return String(value || "").trim();
}

function plainObject(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : null;
}

function apiRoot(ctx) {
  // Upstream path follows the channel base URL. /basamp is only this gateway's public prefix.
  return String(ctx.baseUrl || "").replace(/\/+$/, "");
}

function profileFor(model) {
  return VIDEO_MODELS[model] || { resolutions: RESOLUTIONS, minDuration: 4, maxDuration: 30 };
}

function resolutionFromSize(size) {
  const raw = trimmed(size).toLowerCase();
  if (RESOLUTIONS.indexOf(raw) >= 0) return raw;
  const parts = raw.replace("*", "x").split("x");
  if (parts.length !== 2) return "";
  const width = Number(parts[0]);
  const height = Number(parts[1]);
  if (!(width > 0) || !(height > 0)) return "";
  const max = Math.max(width, height);
  if (max >= 3840) return "4k";
  if (max >= 1920) return "1080p";
  if (max >= 1280) return "720p";
  return "480p";
}

function hasVideo(content) {
  return (
    Array.isArray(content) &&
    content.some(function (item) {
      return item && (item.type === "video_url" || item.role === "reference_video" || Object.prototype.hasOwnProperty.call(item, "video_url"));
    })
  );
}

function hasImage(content) {
  return (
    Array.isArray(content) &&
    content.some(function (item) {
      return item && (item.type === "image_url" || Object.prototype.hasOwnProperty.call(item, "image_url"));
    })
  );
}

function actionFromContent(content) {
  if (hasVideo(content)) return "video_to_video";
  if (hasImage(content)) return "image_to_video";
  return "text_to_video";
}

function modelName(ctx, req) {
  return trimmed((ctx && (ctx.upstreamModel || ctx.model)) || (req && req.model));
}

function outboundResolution(req, model) {
  const metadata = plainObject(req && req.metadata) || {};
  const requested = trimmed(req && (req.resolution || metadata.resolution)) || resolutionFromSize(req && req.size);
  const resolution = requested || "720p";
  const allowed = profileFor(model).resolutions;
  if (allowed.indexOf(resolution) < 0) throw new Error(model + " does not support resolution " + resolution);
  return resolution;
}

function outboundDuration(req, model) {
  const metadata = plainObject(req && req.metadata) || {};
  const raw = req && (req.seconds !== undefined ? req.seconds : req.duration !== undefined ? req.duration : metadata.duration);
  if (raw === undefined || raw === null || raw === "") {
    return model === "doubao-seedance-2-5-260628" ? -1 : 5;
  }
  const duration = Number(raw);
  if (!Number.isInteger(duration)) throw new Error("duration must be an integer");
  if (duration === -1) return -1;
  const profile = profileFor(model);
  if (duration < profile.minDuration || duration > profile.maxDuration) {
    throw new Error(model + " duration must be -1 or between " + profile.minDuration + " and " + profile.maxDuration);
  }
  return duration;
}

function contentItemURL(value) {
  if (plainObject(value) && value.__fileRef) return value;
  if (plainObject(value) && trimmed(value.url)) return trimmed(value.url);
  return trimmed(value);
}

function contentFromRequest(req) {
  if (Array.isArray(req.content)) return req.content.map(normalizeContentItem);
  const metadata = plainObject(req.metadata) || {};
  const content = [];
  const images = [];
  if (Array.isArray(req.images)) {
    for (const image of req.images) if (image) images.push(image);
  }
  if (req.image) images.push(req.image);
  if (req.input_reference) images.push(req.input_reference);
  for (const image of images) {
    const url = contentItemURL(image);
    if (url) content.push({ type: "image_url", image_url: { url: url } });
  }
  if (Array.isArray(metadata.content)) {
    for (const item of metadata.content) {
      const normalized = normalizeContentItem(item);
      if (normalized && normalized.type !== "text") content.push(normalized);
    }
  }
  const prompt = trimmed(req.prompt);
  if (prompt || content.length === 0) content.push({ type: "text", text: prompt });
  return content.filter(function (item) {
    return item && item.type;
  });
}

function normalizeContentItem(item) {
  if (!plainObject(item) || !trimmed(item.type)) return null;
  if (item.type === "text") return { type: "text", text: trimmed(item.text) };
  if (item.type === "image_url") {
    const url = contentItemURL(item.image_url && item.image_url.url !== undefined ? item.image_url.url : item.image_url);
    if (!url) return null;
    const normalized = { type: "image_url", image_url: { url: url } };
    if (trimmed(item.role)) normalized.role = trimmed(item.role);
    return normalized;
  }
  if (item.type === "video_url" || item.type === "audio_url") {
    const field = item.type;
    const url = contentItemURL(item[field] && item[field].url !== undefined ? item[field].url : item[field]);
    if (!url) return null;
    const normalized = { type: item.type };
    normalized[field] = { url: url };
    if (trimmed(item.role)) normalized.role = trimmed(item.role);
    return normalized;
  }
  return item;
}

function upstreamBody(ctx) {
  const req = plainObject(ctx.requestBody) || {};
  const model = modelName(ctx, req);
  if (!model) throw new Error("model is required");
  if (Array.isArray(req.content)) {
    const body = Object.assign({}, req, { model: model, content: contentFromRequest(req) });
    if (!trimmed(body.replyType)) body.replyType = "async";
    if (!body.content.length) throw new Error("content is required");
    return body;
  }
  const metadata = plainObject(req.metadata) || {};
  const content = contentFromRequest(req);
  if (!content.length) throw new Error("content is required");
  const body = {
    model: model,
    content: content,
    resolution: outboundResolution(req, model),
    duration: outboundDuration(req, model),
    replyType: "async",
  };
  const ratio = trimmed(req.ratio || metadata.ratio);
  if (ratio) {
    if (RATIOS.indexOf(ratio) < 0) throw new Error("ratio must be one of " + RATIOS.join(", "));
    body.ratio = ratio;
  }
  for (const key of ["generate_audio", "watermark", "output_format", "return_last_frame", "omni_reference_task_type"]) {
    if (metadata[key] !== undefined) body[key] = metadata[key];
  }
  return body;
}

function authHeaders(ctx) {
  const headers = { Accept: "application/json", Authorization: "Bearer " + ctx.apiKey };
  const incoming = ctx.requestHeaders || {};
  const idempotency = incoming["Idempotency-Key"] || incoming["idempotency-key"];
  if (typeof idempotency === "string" && trimmed(idempotency)) headers["Idempotency-Key"] = trimmed(idempotency);
  return headers;
}

function errorMessage(body, fallback) {
  const payload = plainObject(body) || {};
  const error = plainObject(payload.error);
  return trimmed((error && error.message) || payload.message) || fallback;
}

function responsesInput(req) {
  const texts = [];
  const images = [];
  const input = req.input;
  if (typeof input === "string") texts.push(input);
  else if (Array.isArray(input)) {
    for (const item of input) {
      if (typeof item === "string") {
        texts.push(item);
        continue;
      }
      if (!plainObject(item)) continue;
      const content = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
      for (const part of content) {
        if (typeof part === "string") {
          texts.push(part);
          continue;
        }
        if (!plainObject(part)) continue;
        if (["input_text", "text"].indexOf(part.type) >= 0 && typeof part.text === "string") texts.push(part.text);
        if (["input_image", "image_url"].indexOf(part.type) >= 0) {
          let image = part.image_url;
          if (plainObject(image)) image = image.url;
          if (trimmed(image)) images.push(trimmed(image));
        }
      }
    }
  }
  return {
    prompt: texts.filter(trimmed).join("\n"),
    images: images,
  };
}

function responsesVideoText(ctx) {
  const artifact = ctx && ctx.artifacts && ctx.artifacts.video;
  const url = trimmed(artifact && artifact.url);
  if (!url) throw new Error("video artifact is unavailable");
  const escaped = url.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return '<video controls src="' + escaped + '"></video>';
}

function usageFacts(ctx) {
  const req = plainObject(ctx.requestBody) || {};
  const model = modelName(ctx, req);
  const content = Array.isArray(req.content) ? req.content : contentFromRequest(req);
  let duration = outboundDuration(req, model);
  // The vendor uses -1 for a model-chosen length. Quota has to be a real
  // duration, so that choice reserves the longest video the model can return.
  if (duration < 0) duration = profileFor(model).maxDuration;
  return {
    resolution: outboundResolution(req, model),
    duration: duration,
    video_input: hasVideo(content) ? "video" : "none",
  };
}

export const native = {
  createTask: function (ctx) {
    if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
    const body = ctx.body.value;
    if (!plainObject(body)) throw new Error("request body must be an object");
    const model = trimmed(body.model);
    if (!model) throw new Error("model is required");
    if (!Array.isArray(body.content) || body.content.length === 0) throw new Error("content is required");
    return { kind: "submit", model: model, action: actionFromContent(body.content), requestBody: body };
  },
  taskCreated: function (ctx, task) {
    if (plainObject(task.data)) return Object.assign({}, task.data, { id: task.task_id });
    return { id: task.task_id, object: "video.generation", status: "running" };
  },
  taskStatus: function (ctx, task) {
    if (plainObject(task.data)) return Object.assign({}, task.data, { id: task.task_id });
    const statusMap = { NOT_START: "running", SUBMITTED: "running", QUEUED: "running", IN_PROGRESS: "running", SUCCESS: "succeeded", FAILURE: "failed" };
    const output = { id: task.task_id, object: "video.generation", status: statusMap[task.status] || "running" };
    if (task.status === "FAILURE") output.error = { message: task.fail_reason || "task failed", type: "upstream_error", retryable: false };
    return output;
  },
  error: function (ctx, error) {
    return { error: { message: error.message, type: error.code || "invalid_request", retryable: false } };
  },
};

export function buildSubmitRequest(ctx) {
  const body = upstreamBody(ctx);
  return {
    url: apiRoot(ctx) + "/videos/generations",
    method: "POST",
    headers: Object.assign({ "Content-Type": "application/json" }, authHeaders(ctx)),
    body: body,
    action: actionFromContent(body.content),
    rewriteModel: body.model,
  };
}

export function parseSubmitResponse(ctx, resp) {
  const statusCode = Number(resp && resp.statusCode);
  const body = plainObject(resp && resp.body) || {};
  if (statusCode >= 400) throw new Error(errorMessage(body, "video generation failed"));
  const taskId = trimmed(body.id);
  if (!taskId) throw new Error("task_id is empty");
  if (body.status === "succeeded") return { taskId: taskId, taskData: body, immediate: { status: "SUCCESS", progress: "100%" } };
  if (body.status === "failed") throw new Error(errorMessage(body, "video generation failed"));
  return { taskId: taskId, taskData: body };
}

export function extractUsage(ctx) {
  if (ctx.usagePurpose === "billing_ratios") return null;
  return usageFacts(ctx);
}

export function buildQueryRequest(ctx) {
  return {
    url: apiRoot(ctx) + "/videos/" + encodeURIComponent(ctx.taskId) + "/result",
    method: "GET",
    headers: authHeaders(ctx),
  };
}

export function parseTaskResult(ctx, body, response) {
  const statusCode = Number(response && response.status);
  const payload = plainObject(body) || {};
  if (statusCode === 202 || payload.status === "running" || payload.status === "queued" || payload.status === "pending") {
    return { status: payload.status === "queued" || payload.status === "pending" ? "QUEUED" : "IN_PROGRESS", progress: "50%" };
  }
  if (payload.status === "succeeded") return { status: "SUCCESS", progress: "100%" };
  if (payload.status === "failed" || (statusCode >= 400 && statusCode < 500)) {
    return { status: "FAILURE", progress: "100%", reason: errorMessage(payload, "task failed") };
  }
  return { status: "UNKNOWN", reason: "unrecognized status: " + String(payload.status || statusCode || "") };
}

function artifactData(ctx) {
  const data = (ctx && ctx.data) || {};
  if (data.data && typeof data.data === "object" && data.data.task_id && Object.prototype.hasOwnProperty.call(data.data, "data")) return data.data.data || {};
  return data;
}

export function listArtifacts(task) {
  if (task.status !== "SUCCESS") return [];
  const data = artifactData(task);
  const artifacts = [{ key: "video", type: "video", mimeType: "video/mp4" }];
  if (trimmed(data.last_frame_url)) artifacts.push({ key: "last_frame", type: "image", mimeType: "image/jpeg" });
  return artifacts;
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey === "last_frame") {
    const url = trimmed(artifactData(ctx).last_frame_url);
    if (!url) throw new Error("artifact_not_found");
    return { url: url, method: ctx.clientRequest.method, credentialless: true };
  }
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  return {
    url: apiRoot(ctx) + "/videos/" + encodeURIComponent(ctx.upstreamTaskId) + "/download",
    method: ctx.clientRequest.method,
    headers: authHeaders(ctx),
  };
}

export function extractUsageOnComplete() {
  return {};
}

export const protocols = {
  openai_responses: {
    decodeRequest: function (ctx) {
      if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
      const req = ctx.body.value;
      if (!plainObject(req)) throw new Error("request body must be an object");
      const model = trimmed(req.model);
      if (!model) throw new Error("model is required");
      if (req.input !== undefined && typeof req.input !== "string" && !Array.isArray(req.input)) throw new Error("input must be a string or array");
      if (req.images !== undefined && !Array.isArray(req.images)) throw new Error("images must be an array");
      if (req.metadata !== undefined && !plainObject(req.metadata)) throw new Error("metadata must be an object");
      const input = responsesInput(req);
      const prompt = input.prompt || trimmed(req.prompt);
      const images = [];
      for (const image of [req.image, req.input_reference].concat(req.images || [], input.images)) {
        if (trimmed(image) && images.indexOf(trimmed(image)) < 0) images.push(trimmed(image));
      }
      if (!prompt && images.length === 0) throw new Error("input is required");
      const metadata = Object.assign({}, req.metadata || {});
      const sizeResolution = resolutionFromSize(req.size);
      if (!metadata.resolution && sizeResolution) metadata.resolution = sizeResolution;
      const requestBody = { model: model, prompt: prompt, metadata: metadata };
      if (images.length) requestBody.images = images;
      if (Object.prototype.hasOwnProperty.call(req, "seconds")) requestBody.seconds = req.seconds;
      else if (Object.prototype.hasOwnProperty.call(req, "duration")) requestBody.seconds = req.duration;
      if (Object.prototype.hasOwnProperty.call(req, "size")) requestBody.size = req.size;
      return { kind: "submit", model: model, action: images.length ? "image_to_video" : "text_to_video", requestBody: requestBody };
    },
    renderEvents: function (ctx, task, previousState) {
      const status = String(task.status || "UNKNOWN").toUpperCase();
      const value = Number(String(task.progress || "").replace("%", ""));
      const progress = Number.isFinite(value) && value >= 0 && value <= 100 ? value : null;
      const state = { status: status, progress: progress };
      if (status === "SUCCESS") {
        const text = responsesVideoText(ctx);
        const events = previousState && previousState.status === status ? [] : [{ type: "output", data: text }];
        return { events: events, state: state, done: true };
      }
      if (status === "FAILURE") return { events: [{ type: "error", code: "task_failed", message: task.fail_reason || "task failed" }], state: state, done: true };
      if (previousState && previousState.status === status && previousState.progress === progress) return { events: [], state: state, done: false };
      const event = { type: "progress", message: status.toLowerCase() };
      if (progress !== null) event.progress = progress;
      return { events: [event], state: state, done: false };
    },
    renderFinal: function (ctx) {
      return {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: responsesVideoText(ctx), annotations: [], logprobs: [] }],
          },
        ],
        metadata: { vendor: "anyllm" },
      };
    },
  },
  openai_video: {
    decodeRequest: function (ctx) {
      if (!ctx.body || (ctx.body.kind !== "json" && ctx.body.kind !== "multipart")) throw new Error("JSON or multipart body required");
      if (ctx.body.kind === "json") {
        if (!plainObject(ctx.body.value)) throw new Error("JSON object required");
        const req = Object.assign({}, ctx.body.value);
        if (Array.isArray(req.content)) {
          const model = trimmed(ctx.model || req.model);
          if (!model) throw new Error("model is required");
          return { kind: "submit", model: model, action: actionFromContent(req.content), requestBody: Object.assign({}, req, { model: model }) };
        }
        return protocols.openai_responses.decodeRequest({ body: ctx.body, model: ctx.model });
      }
      const first = function (name) {
        const values = (ctx.body.fields || {})[name] || [];
        if (values.length > 1) throw new Error(name + " must be provided once");
        return values[0];
      };
      const req = {};
      const fields = ctx.body.fields || {};
      for (const name of Object.keys(fields)) req[name] = first(name);
      let hasInputReferenceFile = false;
      for (const file of ctx.body.files || []) {
        if (file.field !== "input_reference") throw new Error("unexpected file field: " + file.field);
        if (hasInputReferenceFile) throw new Error("input_reference must be provided once");
        hasInputReferenceFile = true;
      }
      if (req.seconds !== undefined) req.seconds = Number(req.seconds);
      else if (req.duration !== undefined) req.seconds = Number(req.duration);
      if (hasInputReferenceFile) req.input_reference = { __fileRef: "request_file:input_reference", encoding: "dataUrl", maxBytes: 31457280 };
      req.model = ctx.model;
      return protocols.openai_responses.decodeRequest({ body: { kind: "json", value: req }, model: ctx.model });
    },
    render: function (ctx, task) {
      const statusMap = { NOT_START: "queued", SUBMITTED: "queued", QUEUED: "queued", IN_PROGRESS: "in_progress", SUCCESS: "completed", FAILURE: "failed" };
      const output = {
        id: task.task_id,
        object: "video",
        model: (task.properties || {}).origin_model_name || "",
        status: statusMap[task.status] || "unknown",
        progress: Number(String(task.progress || "0").replace("%", "")),
        created_at: Number(task.created_at || 0),
      };
      if (task.status === "FAILURE") output.error = { code: "video_generation_failed", message: task.fail_reason || "The video generation task failed." };
      return output;
    },
  },
};
