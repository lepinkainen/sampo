export interface Root {
	id: string;
	name: string;
}

export interface TagScore {
	label: string;
	score: number;
}

export interface FileEntry {
	name: string;
	path: string;
	isDir: boolean;
	isZip: boolean;
	size: number;
	modTime: string;
	mediaType: 'image' | 'video' | 'archive' | 'pdf' | 'other';
	hasThumb: boolean;
	hasPerson?: boolean | null;
	tags?: TagScore[] | null;
	ocrText?: string | null;
	sha256?: string | null;
	crc32?: string | null;
	width?: number | null;
	height?: number | null;
	/** Duration in seconds (videos only). */
	duration?: number | null;
}

export interface DuplicateFile {
	rootId: string;
	path: string;
	size?: number;
	width?: number;
	height?: number;
	mtime?: number;
	/** % match vs the group's best file (phash groups only). */
	similarity?: number;
}

export interface DuplicateGroup {
	hash: string;
	/** "sha256" or "crc32" (exact match) or "phash" (visually similar). */
	hashType: string;
	size: number;
	/** phash groups: max Hamming distance to the best file, in bits. */
	maxDistance?: number;
	/** Index into files of the suggested keeper; null/absent = quality tie. */
	keeper?: number | null;
	files: DuplicateFile[];
}

export interface DuplicatesResponse {
	groups: DuplicateGroup[];
}
