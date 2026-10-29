// Downscales to 1600px and re-encodes as JPEG: a 5–12 MB phone photo becomes
// a few hundred KB, well inside Rekognition's limits, always the one content
// type the server signs for, and stripped of EXIF (including GPS).

const MAX_EDGE = 1600;
const JPEG_QUALITY = 0.9;

export class UnreadableImageError extends Error {
  constructor() {
    super("The file isn't an image the browser can open");
    this.name = "UnreadableImageError";
  }
}

export async function preparePhoto(file: Blob): Promise<Blob> {
  let bitmap: ImageBitmap;
  try {
    // Bake in the EXIF rotation before the metadata is dropped.
    bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
  } catch {
    throw new UnreadableImageError(); // e.g. HEIC in a browser that can't decode it
  }

  const scale = Math.min(1, MAX_EDGE / Math.max(bitmap.width, bitmap.height));
  const width = Math.round(bitmap.width * scale);
  const height = Math.round(bitmap.height * scale);

  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Canvas 2D is not available");
  ctx.imageSmoothingQuality = "high";
  ctx.drawImage(bitmap, 0, 0, width, height);
  bitmap.close();

  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/jpeg", JPEG_QUALITY));
  if (!blob) throw new UnreadableImageError();
  return blob;
}
