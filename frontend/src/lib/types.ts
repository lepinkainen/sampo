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
	mediaType: 'image' | 'video' | 'archive' | 'other';
	hasThumb: boolean;
	hasPerson?: boolean | null;
	tags?: TagScore[] | null;
	ocrText?: string | null;
	sha256?: string | null;
	crc32?: string | null;
}

export interface DuplicateFile {
	rootId: string;
	path: string;
}

export interface DuplicateGroup {
	hash: string;
	hashType: string;
	size: number;
	files: DuplicateFile[];
}

export interface DuplicatesResponse {
	groups: DuplicateGroup[];
}
