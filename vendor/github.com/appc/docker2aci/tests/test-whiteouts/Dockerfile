FROM busybox
COPY check.sh /

RUN echo yes > layer0-file1 ; ln layer0-file1 layer0-file2 ; ln layer0-file1 layer0-file3
RUN echo yes > layer1-file1 ; ln layer1-file1 layer1-file2 ; ln layer1-file1 layer1-file3
RUN echo yes > layer2-file1 ; ln layer2-file1 layer2-file2 ; ln layer2-file1 layer2-file3
RUN echo yes > layer3-file1 ; ln layer3-file1 layer3-file2 ; ln layer3-file1 layer3-file3
RUN rm -f layer1-file1 layer2-file2 layer3-file3

RUN echo yes > layer4-file1 ; ln layer4-file1 layer4-file2 ; ln layer4-file1 layer4-file3
RUN echo yes > layer5-file1 ; ln layer5-file1 layer5-file2 ; ln layer5-file1 layer5-file3
RUN echo yes > layer6-file1 ; ln layer6-file1 layer6-file2 ; ln layer6-file1 layer6-file3
RUN rm -f layer4-file2 layer5-file1 layer6-file1 layer4-file3 layer5-file3 layer6-file2

RUN echo OLD > layer10-file1 ; ln layer10-file1 layer10-file2 ; ln layer10-file1 layer10-file3
RUN echo NEW > layer10-file1 ; ln -f layer10-file1 layer10-file2 ; ln -f layer10-file1 layer10-file3 ; echo foo > foo

RUN echo line1 >  layer11-file1 ; ln layer11-file1 layer11-file2 ; ln layer11-file1 layer11-file3
RUN echo line2 >> layer11-file1

CMD /check.sh 2>&1
