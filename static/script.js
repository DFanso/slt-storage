document.addEventListener('DOMContentLoaded', () => {
    // --- DOM Elements ---
    const fileInput = document.getElementById('file-input');
    const dropZone = document.getElementById('drop-zone');
    const selectionContainer = document.getElementById('selection-container');
    const selectionList = document.getElementById('selection-list');
    const uploadBtn = document.getElementById('upload-btn');
    const clearSelectionBtn = document.getElementById('clear-selection-btn');
    const fileList = document.getElementById('file-list');
    const progressBar = document.querySelector('.progress');
    const progressText = document.querySelector('.progress-text');
    const loadingSpinner = document.getElementById('loading-spinner');
    const breadcrumb = document.getElementById('breadcrumb');
    const createFolderBtn = document.getElementById('create-folder-btn');
    const uploadContainer = document.getElementById('upload-container');

    // --- State ---
    let currentPath = '/';
    let filesToUpload = [];
    const chunkSize = 500 * 1024 * 1024; // 500MB

    // --- Functions ---

    const showSpinner = () => loadingSpinner.classList.remove('hidden');
    const hideSpinner = () => loadingSpinner.classList.add('hidden');

    const updateProgress = (progress) => {
        progressBar.style.width = `${progress.toFixed(2)}%`;
        progressText.textContent = `${Math.round(progress)}%`;
    };

    const resetProgress = () => {
        updateProgress(0);
        progressText.textContent = '';
    };

    const renderSelectionList = () => {
        selectionList.innerHTML = '';
        filesToUpload.forEach(file => {
            const li = document.createElement('li');
            li.className = 'flex items-center p-2 bg-gray-100 rounded';
            li.innerHTML = `
                <ion-icon name="document-text-outline" class="text-xl text-gray-600 mr-3"></ion-icon>
                <span class="text-sm font-medium text-gray-800">${file.name}</span>
            `;
            selectionList.appendChild(li);
        });
    };

    const clearSelection = () => {
        filesToUpload = [];
        fileInput.value = ''; // Reset the file input
        selectionContainer.classList.add('hidden');
        dropZone.classList.remove('hidden');
        resetProgress();
    };

    const renderBreadcrumb = () => {
        breadcrumb.innerHTML = '';
        const parts = currentPath.split('/').filter(p => p);
        let path = '/';

        const rootLink = document.createElement('a');
        rootLink.href = '#';
        rootLink.className = 'hover:text-blue-500 transition-colors';
        rootLink.textContent = 'Root';
        rootLink.dataset.path = '/';
        breadcrumb.appendChild(rootLink);

        parts.forEach(part => {
            path += part + '/';
            const separator = document.createElement('span');
            separator.className = 'text-gray-400';
            separator.textContent = '/';
            breadcrumb.appendChild(separator);

            const link = document.createElement('a');
            link.href = '#';
            link.className = 'hover:text-blue-500 transition-colors';
            link.textContent = part;
            link.dataset.path = path;
            breadcrumb.appendChild(link);
        });
    };

    const fetchFiles = async (path) => {
        currentPath = path;
        renderBreadcrumb();
        showSpinner();
        fileList.innerHTML = '<li class="text-gray-500">Loading files...</li>';
        try {
            const response = await fetch(`/api/files?path=${encodeURIComponent(path)}`);
            if (!response.ok) throw new Error('Failed to fetch files.');
            const files = await response.json();

            fileList.innerHTML = '';
            if (!files || files.length === 0) {
                fileList.innerHTML = '<li class="text-gray-500">This folder is empty.</li>';
                return;
            }

            files.forEach(file => {
                const li = document.createElement('li');
                li.className = 'file-item flex justify-between items-center p-3 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors cursor-pointer';
                li.dataset.path = file.path;
                li.dataset.isDir = file.isDir;
                
                const iconName = file.isDir ? 'folder-outline' : 'document-outline';
                
                li.innerHTML = `
                    <div class="flex items-center space-x-3 pointer-events-none">
                        <ion-icon name="${iconName}" class="text-2xl text-gray-500"></ion-icon>
                        <span class="font-medium text-gray-700">${file.name}</span>
                    </div>
                    <div class="flex items-center space-x-4">
                        <a href="/api/download?path=${encodeURIComponent(file.path)}" class="download-link text-blue-500 hover:text-blue-700 transition-colors" title="Download" style="${file.isDir ? 'display: none;' : ''}">
                            <ion-icon name="download-outline" class="text-2xl"></ion-icon>
                        </a>
                        <button data-path="${file.path}" class="delete-button text-red-500 hover:text-red-700 transition-colors" title="Delete">
                            <ion-icon name="trash-outline" class="text-2xl"></ion-icon>
                        </button>
                    </div>
                `;
                fileList.appendChild(li);
            });
        } catch (error) {
            console.error('Error fetching files:', error);
            fileList.innerHTML = '<li class="text-red-500">Could not load files.</li>';
        } finally {
            hideSpinner();
        }
    };

    const uploadFile = async (file) => {
        resetProgress();
        const uploadID = `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
        const wsProtocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
        const ws = new WebSocket(`${wsProtocol}://${window.location.host}/ws/progress?id=${uploadID}`);

        ws.onopen = () => console.log(`WebSocket connected for upload: ${uploadID}`);
        ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            const progress = (data.totalWritten / data.totalSize) * 100;
            updateProgress(progress);
        };
        ws.onerror = (error) => console.error('WebSocket error:', error);
        ws.onclose = () => {
            console.log('WebSocket disconnected.');
            // This will be handled by the upload queue now
        };

        const totalChunks = Math.ceil(file.size / chunkSize);
        for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
            const start = chunkIndex * chunkSize;
            const chunk = file.slice(start, start + chunkSize);
            const formData = new FormData();
            formData.append('file', chunk);
            formData.append('chunkIndex', chunkIndex.toString());
            formData.append('originalFilename', file.name);
            formData.append('uploadID', uploadID);
            formData.append('totalSize', file.size.toString());
            formData.append('startOffset', start.toString());
            formData.append('currentPath', currentPath);

            const response = await fetch('/api/upload', { method: 'POST', body: formData });
            if (!response.ok) {
                console.error('Upload chunk failed:', await response.text());
                ws.close();
                return;
            }
        }
        ws.close();
    };

    const handleFiles = (newFiles) => {
        const uniqueNewFiles = Array.from(newFiles).filter(file => !filesToUpload.some(existingFile => existingFile.name === file.name && existingFile.size === file.size));

        if (uniqueNewFiles.length > 0) {
            filesToUpload.push(...uniqueNewFiles);
        }

        if (filesToUpload.length > 0) {
            renderSelectionList();
            dropZone.classList.add('hidden');
            selectionContainer.classList.remove('hidden');
        }
    };

    // --- Event Listeners ---
    
    dropZone.addEventListener('click', () => fileInput.click());
    fileInput.addEventListener('change', (e) => handleFiles(e.target.files));

    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
        uploadContainer.addEventListener(eventName, e => {
            e.preventDefault();
            e.stopPropagation();
        });
    });

    ['dragenter', 'dragover'].forEach(eventName => {
        uploadContainer.addEventListener(eventName, () => {
            uploadContainer.classList.add('bg-blue-50');
            if (filesToUpload.length === 0) {
                dropZone.classList.add('border-blue-500');
            }
        });
    });

    ['dragleave', 'drop'].forEach(eventName => {
        uploadContainer.addEventListener(eventName, () => {
            uploadContainer.classList.remove('bg-blue-50');
            if (filesToUpload.length === 0) {
                dropZone.classList.remove('border-blue-500');
            }
        });
    });

    uploadContainer.addEventListener('drop', (e) => {
        handleFiles(e.dataTransfer.files);
    });

    breadcrumb.addEventListener('click', (e) => {
        if (e.target.tagName === 'A') {
            e.preventDefault();
            fetchFiles(e.target.dataset.path);
        }
    });

    fileList.addEventListener('click', async (e) => {
        const item = e.target.closest('.file-item');
        if (!item) return;

        const isDir = item.dataset.isDir === 'true';
        const path = item.dataset.path;

        if (e.target.closest('.delete-button')) {
            if (confirm(`Are you sure you want to delete ${path}?`)) {
                showSpinner();
                try {
                    await fetch(`/api/files?path=${encodeURIComponent(path)}`, { method: 'DELETE' });
                } catch (error) {
                    console.error('Failed to delete:', error);
                    alert('Error: Could not delete the item.');
                } finally {
                    fetchFiles(currentPath);
                }
            }
        } else if (e.target.closest('.download-link')) {
            // Let the anchor tag handle the download
        } else if (isDir) {
            fetchFiles(path);
        }
    });

    createFolderBtn.addEventListener('click', async () => {
        const folderName = prompt('Enter a name for the new folder:');
        if (folderName) {
            const newPath = (currentPath.endsWith('/') ? currentPath : currentPath + '/') + folderName;
            const formData = new FormData();
            formData.append('path', newPath);
            showSpinner();
            try {
                const response = await fetch('/api/folders', { method: 'POST', body: formData });
                if (!response.ok) throw new Error('Failed to create folder.');
            } catch (error) {
                console.error('Error creating folder:', error);
                alert('Error: Could not create folder.');
            } finally {
                fetchFiles(currentPath);
            }
        }
    });

    clearSelectionBtn.addEventListener('click', clearSelection);

    uploadBtn.addEventListener('click', async () => {
        uploadBtn.disabled = true;
        clearSelectionBtn.disabled = true;

        for (const file of filesToUpload) {
            await uploadFile(file);
        }

        uploadBtn.disabled = false;
        clearSelectionBtn.disabled = false;
        clearSelection();
        fetchFiles(currentPath);
    });

    // --- Initial Load ---
    fetchFiles('/');
}); 