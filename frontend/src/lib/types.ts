export interface Root {
	id: string;
	name: string;
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
}
