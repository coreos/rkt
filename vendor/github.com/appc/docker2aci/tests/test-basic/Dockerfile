FROM busybox
COPY check.sh /
RUN echo file1 > file1 ; ln file1 file2
RUN echo file3 > file3
RUN echo file4 > file4
CMD /check.sh 2>&1
