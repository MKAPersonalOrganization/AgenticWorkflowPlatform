/**
 * PDF Viewer Component
 * Handles PDF rendering using PDF.js
 * 
 * @param {string} url - The URL of the PDF to load
 */
export const pdfViewer = (url) => ({
    pdfDoc: null,
    pageNum: 1,
    numPages: 0,
    scale: 1.0,
    isLoading: true,
    error: null,
    renderTask: null,

    async init() {
        if (!url) {
            this.error = "No PDF URL provided";
            this.isLoading = false;
            return;
        }

        try {
            const pdfjsLib = window['pdfjs-dist/build/pdf'];
            if (!pdfjsLib) throw new Error("PDF.js library not found");

            const loadingTask = pdfjsLib.getDocument(url);
            this.pdfDoc = await loadingTask.promise;
            this.numPages = this.pdfDoc.numPages;
            this.isLoading = false;

            // Use nextTick to ensure canvas ref is available
            this.$nextTick(() => this.renderPage());
        } catch (err) {
            console.error("PDF load error:", err);
            this.error = err.message || "Failed to load document";
            this.isLoading = false;
        }
    },

    async renderPage() {
        if (!this.pdfDoc || !this.$refs.pdfCanvas) return;

        // Cancel existing render task if any
        if (this.renderTask) {
            this.renderTask.cancel();
        }

        try {
            const page = await this.pdfDoc.getPage(this.pageNum);
            const canvas = this.$refs.pdfCanvas;
            const context = canvas.getContext('2d');

            // DRP handling for high resolution screens
            const dpr = window.devicePixelRatio || 1;
            const viewport = page.getViewport({
                scale: this.scale * dpr
            });

            canvas.height = viewport.height;
            canvas.width = viewport.width;

            // Scaling for CSS
            canvas.style.width = (viewport.width / dpr) + "px";
            canvas.style.height = (viewport.height / dpr) + "px";

            const renderContext = {
                canvasContext: context,
                viewport: viewport
            };

            this.renderTask = page.render(renderContext);
            await this.renderTask.promise;
        } catch (err) {
            if (err.name !== 'RenderingCancelledException') {
                console.error("Canvas render error:", err);
            }
        }
    },

    nextPage() {
        if (this.pageNum < this.numPages) {
            this.pageNum++;
            this.renderPage();
        }
    },

    prevPage() {
        if (this.pageNum > 1) {
            this.pageNum--;
            this.renderPage();
        }
    },

    zoomIn() {
        this.scale = Math.min(3.0, this.scale + 0.25);
        this.renderPage();
    },

    zoomOut() {
        this.scale = Math.max(0.5, this.scale - 0.25);
        this.renderPage();
    },

    resetZoom() {
        this.scale = 1.0;
        this.renderPage();
    }
});
