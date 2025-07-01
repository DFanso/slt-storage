document.addEventListener('DOMContentLoaded', () => {
    const fileInput = document.getElementById('file-input');
    const uploadButton = document.getElementById('upload-button');
    const fileList = document.getElementById('file-list');
    const progressBar = document.querySelector('.progress');

    const webdavUrl = 'https://storage.slt.lk/drive/remote.php/webdav/';
    const chunkSize = 500 * 1024 * 1024; // 500MB

    const fetchFiles = async () => {
        const response = await fetch('/api/files');
        const files = await response.json();
        fileList.innerHTML = '';
        files.forEach(file => {
            const li = document.createElement('li');
            li.className = 'flex justify-between items-center p-2 hover:bg-gray-50';
            li.innerHTML = `
                <span>${file}</span>
                <div>
                    <a href="/api/download/${file}" class="text-blue-500 hover:text-blue-700 mr-2">Download</a>
                    <button data-filename="${file}" class="delete-button text-red-500 hover:text-red-700">Delete</button>
                </div>
            `;
            fileList.appendChild(li);
        });
    };

    const uploadFile = async (file) => {
        const totalChunks = Math.ceil(file.size / chunkSize);
        progressBar.style.width = '0%';
        progressBar.textContent = '0%';

        const uploadID = Date.now().toString() + Math.random().toString(36).substring(2);
        const wsProtocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
        const wsURL = `${wsProtocol}://${window.location.host}/ws/progress?id=${uploadID}`;
        const ws = new WebSocket(wsURL);

        ws.onopen = () => {
            console.log('WebSocket connected for upload:', uploadID);
        };
        
        ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            const progress = (data.totalWritten / data.totalSize) * 100;
            progressBar.style.width = `${progress.toFixed(2)}%`;
            progressBar.textContent = `${Math.round(progress)}%`;
        };
        
        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        ws.onclose = () => {
            console.log('WebSocket disconnected.');
            setTimeout(() => {
                progressBar.textContent = '';
                progressBar.style.width = '0%';
            }, 3000);
            fetchFiles();
        };

        for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
            const start = chunkIndex * chunkSize;
            const end = Math.min(start + chunkSize, file.size);
            const chunk = file.slice(start, end);

            const formData = new FormData();
            formData.append('file', chunk);
            formData.append('chunkIndex', chunkIndex.toString());
            formData.append('originalFilename', file.name);
            formData.append('uploadID', uploadID);
            formData.append('totalSize', file.size.toString());
            formData.append('startOffset', start.toString());

            const response = await fetch('/api/upload', {
                method: 'POST',
                body: formData
            });

            if (!response.ok) {
                console.error('Upload chunk failed:', await response.text());
                ws.close();
                return;
            }
        }
        ws.close();
    };

    uploadButton.addEventListener('click', () => {
        const files = fileInput.files;
        if (files.length === 0) {
            alert('Please select a file to upload.');
            return;
        }
        for (const file of files) {
            uploadFile(file);
        }
    });

    fileList.addEventListener('click', async (e) => {
        if (e.target.classList.contains('delete-button')) {
            const filename = e.target.dataset.filename;
            if (confirm(`Are you sure you want to delete ${filename}?`)) {
                await fetch(`/api/files/${filename}`, { method: 'DELETE' });
                fetchFiles();
            }
        }
    });

    fetchFiles();
}); 