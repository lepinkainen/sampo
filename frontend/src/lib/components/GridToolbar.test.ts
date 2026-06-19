import { render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import GridToolbar from './GridToolbar.svelte';

function makeProps(over: Record<string, unknown> = {}) {
	return {
		rootId: 'root-0',
		rootName: 'Sample',
		pathSegments: [],
		backgroundValidating: false,
		selectionSize: 0,
		hasClipboard: false,
		searchActive: false,
		searchQuery: '',
		searchLoading: false,
		searchResultCount: 0,
		analysisSettings: null,
		analysisSettingsSaving: false,
		scanStatus: null,
		classifyScanStatus: null,
		ocrScanStatus: null,
		analyzeScanStatus: null,
		filterPeople: false,
		filterTag: '',
		availableTags: [],
		viewMode: 'grid' as const,
		thumbSize: 'medium' as const,
		onNavigate: vi.fn(),
		onOpenSearch: vi.fn(),
		onCloseSearch: vi.fn(),
		onSearchInput: vi.fn(),
		onCut: vi.fn(),
		onCopy: vi.fn(),
		onPaste: vi.fn(),
		onRename: vi.fn(),
		onDelete: vi.fn(),
		onToggleAutoBrowse: vi.fn(),
		onToggleFilter: vi.fn(),
		onScan: vi.fn(),
		onClassify: vi.fn(),
		onOCR: vi.fn(),
		onReanalyze: vi.fn(),
		onFindDuplicates: vi.fn(),
		onTagFilter: vi.fn(),
		onViewMode: vi.fn(),
		onThumbSize: vi.fn(),
		...over,
	};
}

describe('GridToolbar', () => {
	it('renders breadcrumb segments', () => {
		render(GridToolbar, makeProps({ pathSegments: ['photos', '2024'] }));
		expect(screen.getByText('Sample')).toBeInTheDocument();
		expect(screen.getByText('photos')).toBeInTheDocument();
		expect(screen.getByText('2024')).toBeInTheDocument();
	});

	it('disables file-op buttons with no selection', () => {
		render(GridToolbar, makeProps({ selectionSize: 0, hasClipboard: false }));
		expect(screen.getByTitle('Cut (Ctrl+X)')).toBeDisabled();
		expect(screen.getByTitle('Copy (Ctrl+C)')).toBeDisabled();
		expect(screen.getByTitle('Paste (Ctrl+V)')).toBeDisabled();
		expect(screen.getByTitle('Rename (F2)')).toBeDisabled();
		expect(screen.getByTitle('Delete')).toBeDisabled();
	});

	it('enables cut/copy/delete with a selection but rename only for one', () => {
		render(GridToolbar, makeProps({ selectionSize: 2 }));
		expect(screen.getByTitle('Cut (Ctrl+X)')).toBeEnabled();
		expect(screen.getByTitle('Delete')).toBeEnabled();
		expect(screen.getByTitle('Rename (F2)')).toBeDisabled();
	});

	it('invokes callbacks on button clicks', () => {
		const onScan = vi.fn();
		const onFindDuplicates = vi.fn();
		render(GridToolbar, makeProps({ onScan, onFindDuplicates }));
		screen.getByTitle('Scan for people').click();
		screen.getByTitle('Find duplicates').click();
		expect(onScan).toHaveBeenCalledOnce();
		expect(onFindDuplicates).toHaveBeenCalledOnce();
	});

	it('disables scan buttons while their scan is running', () => {
		render(
			GridToolbar,
			makeProps({
				scanStatus: { running: true, total: 5, completed: 1, errors: 0 },
			}),
		);
		expect(screen.getByTitle('Scan for people')).toBeDisabled();
		expect(screen.getByText('Scanning 1/5')).toBeInTheDocument();
	});
});
